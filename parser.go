package parser

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strconv"
	"strings"
	"unicode"

	"github.com/nickastewart/muttley-parser/model"
	"golang.org/x/net/html"
)

func ParseFile(reader io.Reader) (*model.Event, error) {
	message, err := mail.ReadMessage(reader)
	if err != nil {
		return nil, fmt.Errorf("read message: %w", err)
	}

	rawHTML, err := extractHTML(message.Header, message.Body)
	if err != nil {
		return nil, err
	}

	date, err := formatEventDate(message.Header.Get("Date"))
	if err != nil {
		return nil, err
	}

	return parseEvent(rawHTML, message.Header.Get("Subject"), date)
}

func formatEventDate(dateHeader string) (string, error) {
	parsed, err := mail.ParseDate(dateHeader)
	if err != nil {
		return "", fmt.Errorf("parse date: %w", err)
	}
	return parsed.Format("2 Jan 2006"), nil
}

func extractHTML(header mail.Header, body io.Reader) (string, error) {
	mediaType, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		return "", fmt.Errorf("content type: %w", err)
	}
	return htmlFromPart(mediaType, params, header.Get("Content-Transfer-Encoding"), body)
}

func htmlFromPart(mediaType string, params map[string]string, encoding string, body io.Reader) (string, error) {
	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return "", fmt.Errorf("multipart boundary missing")
		}
		return htmlFromMultipart(multipart.NewReader(body, boundary))
	}

	if mediaType != "text/html" && mediaType != "text/plain" {
		return "", fmt.Errorf("html part not found")
	}

	decoded, err := decodeTransfer(encoding, body)
	if err != nil {
		return "", err
	}
	if mediaType == "text/html" || strings.Contains(strings.ToLower(decoded), "<html>") {
		return decoded, nil
	}
	return "", fmt.Errorf("html part not found")
}

func htmlFromMultipart(reader *multipart.Reader) (string, error) {
	var plainFallback string
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read mime part: %w", err)
		}

		mediaType, params, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			part.Close()
			continue
		}

		if mediaType == "text/html" {
			decoded, err := decodeTransfer(part.Header.Get("Content-Transfer-Encoding"), part)
			part.Close()
			if err != nil {
				return "", err
			}
			return decoded, nil
		}

		if strings.HasPrefix(mediaType, "multipart/") {
			nested, err := htmlFromPart(mediaType, params, part.Header.Get("Content-Transfer-Encoding"), part)
			part.Close()
			if err == nil {
				return nested, nil
			}
			continue
		}

		if mediaType == "text/plain" && plainFallback == "" {
			decoded, err := decodeTransfer(part.Header.Get("Content-Transfer-Encoding"), part)
			part.Close()
			if err != nil {
				return "", err
			}
			if strings.Contains(strings.ToLower(decoded), "<html>") {
				plainFallback = decoded
			}
			continue
		}

		part.Close()
	}

	if plainFallback != "" {
		return plainFallback, nil
	}
	return "", fmt.Errorf("html part not found")
}

func decodeTransfer(encoding string, body io.Reader) (string, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "7bit", "8bit", "binary":
		data, err := io.ReadAll(body)
		if err != nil {
			return "", fmt.Errorf("read body: %w", err)
		}
		return string(data), nil
	case "quoted-printable":
		data, err := io.ReadAll(quotedprintable.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("quoted-printable: %w", err)
		}
		return string(data), nil
	case "base64":
		raw, err := io.ReadAll(body)
		if err != nil {
			return "", fmt.Errorf("read body: %w", err)
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, string(raw)))
		if err != nil {
			return "", fmt.Errorf("base64: %w", err)
		}
		return string(decoded), nil
	default:
		return "", fmt.Errorf("unsupported transfer encoding %q", encoding)
	}
}

func parseEvent(rawHTML string, subject string, date string) (*model.Event, error) {
	root, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	name, err := textByID(root, "lblName")
	if err != nil {
		return nil, fmt.Errorf("driver name: %w", err)
	}
	positionText, err := textByID(root, "lblPosition")
	if err != nil {
		return nil, fmt.Errorf("driver position: %w", err)
	}
	position, err := parsePosition(positionText)
	if err != nil {
		return nil, fmt.Errorf("driver position: %w", err)
	}
	raceType, err := textByID(root, "lblHeatType")
	if err != nil {
		return nil, fmt.Errorf("race type: %w", err)
	}
	results := findByID(root, "dg")
	if results == nil {
		return nil, fmt.Errorf("results table not found")
	}
	times, err := parseResults(results)
	if err != nil {
		return nil, err
	}

	return &model.Event{
		Date:        date,
		Location:    getLocationFromSubject(subject),
		RaceType:    strings.ReplaceAll(raceType, "`", ""),
		DriverInfo:  model.DriverInfo{Name: name, Position: position},
		DriverTimes: times,
	}, nil
}

func textByID(root *html.Node, id string) (string, error) {
	node := findByID(root, id)
	if node == nil {
		return "", fmt.Errorf("%s not found", id)
	}
	text := strings.TrimSpace(textContent(node))
	if text == "" {
		return "", fmt.Errorf("%s is empty", id)
	}
	return text, nil
}

func parseResults(table *html.Node) ([]model.DriverTime, error) {
	rows := searchHtml(table, "tr", nil)
	if len(rows) == 0 {
		return nil, fmt.Errorf("results table has no rows")
	}

	var times []model.DriverTime
	for _, row := range rows[1:] {
		cells := extractTextIter(row)
		if len(cells) == 0 {
			continue
		}
		parsed, err := parseResultRow(cells)
		if err != nil {
			return nil, err
		}
		times = append(times, parsed)
	}
	if len(times) == 0 {
		return nil, fmt.Errorf("results table has no rows")
	}
	return times, nil
}

func parseResultRow(cells []string) (model.DriverTime, error) {
	if len(cells) != 7 {
		return model.DriverTime{}, fmt.Errorf("result row has %d cells, want 7", len(cells))
	}

	pos, err := strconv.Atoi(cells[0])
	if err != nil {
		return model.DriverTime{}, fmt.Errorf("result position %q", cells[0])
	}
	best, err := convertFromStringTime(cells[3])
	if err != nil {
		return model.DriverTime{}, err
	}
	laps, err := strconv.Atoi(cells[4])
	if err != nil {
		return model.DriverTime{}, fmt.Errorf("lap count %q", cells[4])
	}
	avg, err := convertFromStringTime(cells[5])
	if err != nil {
		return model.DriverTime{}, err
	}

	return model.DriverTime{
		Pos:    pos,
		Kart:   cells[1],
		Racer:  cells[2],
		Best:   best,
		NoLaps: laps,
		Avg:    avg,
		Gap:    cells[6],
	}, nil
}

func getLocationFromSubject(subject string) string {
	if strings.Contains(subject, "Daytona Milton Keynes") {
		return "Daytona Milton Keynes"
	}
	if strings.Contains(subject, "Daytona Sandown Park") {
		return "Daytona Sandown Park"
	}
	return "Unrecognised"
}

func findByID(n *html.Node, id string) *html.Node {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode {
		for _, attr := range n.Attr {
			if attr.Key == "id" && attr.Val == id {
				return n
			}
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := findByID(child, id); found != nil {
			return found
		}
	}
	return nil
}

func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return b.String()
}

func searchHtml(n *html.Node, term string, result []*html.Node) []*html.Node {
	if n.Type == html.ElementNode && n.Data == term {
		result = append(result, n)
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		result = searchHtml(child, term, result)
	}
	return result
}

func extractTextIter(n *html.Node) []string {
	var data []string
	stack := []*html.Node{n}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node.Type == html.TextNode {
			txt := strings.TrimSpace(node.Data)
			if txt != "" {
				data = append(data, txt)
			}
		}
		for child := node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	return data
}

func convertFromStringTime(time string) (int, error) {
	parts := strings.Split(time, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("lap time %q", time)
	}
	minutes, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("lap time %q", time)
	}
	seconds, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("lap time %q", time)
	}
	milli, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, fmt.Errorf("lap time %q", time)
	}
	return (minutes * 60 * 1000) + (seconds * 1000) + milli, nil
}

func parsePosition(position string) (int, error) {
	digits := stripPosition(strings.TrimSpace(position))
	if digits == "" {
		return 0, fmt.Errorf("position %q", position)
	}
	value, err := strconv.Atoi(digits)
	if err != nil {
		return 0, fmt.Errorf("position %q", position)
	}
	return value, nil
}

func stripPosition(position string) string {
	for index, char := range position {
		if !unicode.IsDigit(char) {
			return position[:index]
		}
	}
	return position
}
