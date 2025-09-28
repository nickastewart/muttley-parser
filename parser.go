package parser

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/nickastewart/racer-parser/model"
	"golang.org/x/net/html"
)

func ParseFromPath(path string) model.Event {
	data, err := os.ReadFile(path)

	if err != nil {
		log.Panic(err)
	}

	return ParseFile(data)
}

func ParseFile(data []byte) model.Event {

	message, err := mail.ReadMessage(bytes.NewReader(data))

	if err != nil {
		log.Panic(err)
	}

	buf := new(strings.Builder)

	_, err = io.Copy(buf, message.Body)

	if err != nil {
		log.Panic(err)
	}

	body := buf.String()

	var html string = getHtml(body)
	var event model.Event = parseEvent(html, message.Header.Get("Subject"), message.Header.Get("Date"))
	return event

}

func getHtml(data string) string {
	scanner := bufio.NewScanner(strings.NewReader(data))

	var isHtml bool = false
	var htmlStrings []string
	for scanner.Scan() {
		var line string = scanner.Text()

		if strings.TrimRight(line, "\n") == "<html>" {
			isHtml = true
		}

		if isHtml {
			htmlStrings = append(htmlStrings, line)
			if strings.TrimRight(line, "\n") == "</html>" {
				isHtml = false
			}
		}
	}
	return strings.Join(htmlStrings, "")
}

func parseEvent(rawHtml string, subject string, date string) model.Event {
	rootNode, _ := html.Parse(strings.NewReader(rawHtml))
	var tables []*html.Node = searchHtml(rootNode, "table", []*html.Node{})
	var driverInfoHtml []*html.Node = searchHtml(tables[0], "tr", []*html.Node{})

	var driverInfo model.DriverInfo = model.DriverInfo{
		Name: extractTextIter(driverInfoHtml[3])[1],
		Position: stripPosition(extractTextIter(driverInfoHtml[4])[2]),
	}

	var raceInfo model.Event = model.Event{
		Date:     stripTime(date),
		Location: getLocationFromSubject(subject),
		RaceType: extractTextIter(driverInfoHtml[6])[1],
	}

	var raceData []model.DriverTime
	var firstRow = true
	for _, row := range searchHtml(tables[2], "tr", []*html.Node{}) {
		if firstRow == true {
			firstRow = false
			continue
		}
		row := extractTextIter(row)
		if row[0] == driverInfo.Position {
			data := model.DriverTime{
				Pos:    row[0],
				Kart:   row[1],
				Racer:  driverInfo.Name,
				Best:   convertFromStringTime(row[2]),
				NoLaps: row[3],
				Avg:    convertFromStringTime(row[4]),
				Gap:    row[5],
			}
			raceData = append(raceData, data)
		} else {
			data := model.DriverTime{
				Pos:    row[0],
				Kart:   row[1],
				Racer:  row[2],
				Best:   convertFromStringTime(row[3]),
				NoLaps: row[4],
				Avg:    convertFromStringTime(row[5]),
				Gap:    row[6],
			}
			raceData = append(raceData, data)
		}
	}
	raceInfo.DriverTimes = raceData
	raceInfo.DriverInfo = driverInfo
	return raceInfo
}

func stripTime(date string) string {
	return strings.Trim(date[0:12], " ")
}

func getLocationFromSubject(subject string) string {
	if strings.Contains(subject, "Milton Keynes") {
		return "Daytona Milton Keynes"
	}
	return "Unrecognised"
}

func searchHtml(n *html.Node, term string, result []*html.Node) []*html.Node {
	if n.Type == html.ElementNode && n.Data == term {
		result = append(result, n)
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result = searchHtml(c, term, result)
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

		for c := node.LastChild; c != nil; c = c.PrevSibling {
			stack = append(stack, c)
		}
	}
	return data
}

func convertFromStringTime(time string) int {
	var parts []string = strings.Split(time, ":")
	minutes, err := strconv.Atoi(parts[0])
	if err != nil {
		log.Panic(err)
	}
	seconds, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Panic(err)
	}
	milli, err := strconv.Atoi(parts[2])
	if err != nil {
		log.Panic(err)
	}

	return (minutes * 60 * 1000) + (seconds * 1000) + milli
}

func stripPosition(position string) string {
	for index, char := range position {
		if !unicode.IsDigit(char) {
			return position[:index]
		}
	}
	return position
}
