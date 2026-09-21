package parser

import (
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"unicode"

	"github.com/nickastewart/muttley-parser/model"
	"golang.org/x/net/html"
)

func formatEventDate(dateHeader string) (string, error) {
	parsed, err := mail.ParseDate(dateHeader)
	if err != nil {
		return "", fmt.Errorf("parse date: %w", err)
	}
	return parsed.Format("2 Jan 2006"), nil
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
