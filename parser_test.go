package parser

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/nickastewart/muttley-parser/model"
)

func TestParseFileFixtures(t *testing.T) {
	tests := []struct {
		file     string
		location string
		date     string
		raceType string
		driver   string
		position int
		rows     []model.DriverTime
	}{
		{
			file:     "2024-07-16-Milton-Keynes.eml",
			location: "Daytona Milton Keynes",
			date:     "16 Jul 2024",
			raceType: "(2024 SODI 40min. Race)",
			driver:   "Nick Stewart",
			position: 17,
			rows: []model.DriverTime{
				{Pos: 1, Kart: "23", Racer: "H - Craig McAl...", Best: 75175, NoLaps: 32, Avg: 76539, Gap: "-"},
				{Pos: 17, Kart: "8", Racer: "Nick Stewart", Best: 79806, NoLaps: 28, Avg: 87336, Gap: "4L"},
				{Pos: 19, Kart: "22", Racer: "Douglas Willin...", Best: 77372, NoLaps: 4, Avg: 78905, Gap: "28L"},
			},
		},
		{
			file:     "2026-03-05-sandown.eml",
			location: "Daytona Sandown Park",
			date:     "5 Mar 2026",
			raceType: "(DMAX Sprint Race)",
			driver:   "Nicholas Stewart",
			position: 14,
			rows: []model.DriverTime{
				{Pos: 1, Kart: "56", Racer: "H - Louie Pate...", Best: 46966, NoLaps: 25, Avg: 47517, Gap: "-"},
				{Pos: 2, Kart: "143", Racer: "Andrew Lansdown", Best: 46757, NoLaps: 25, Avg: 47566, Gap: "1.725"},
				{Pos: 14, Kart: "54", Racer: "Nicholas Stewart", Best: 50844, NoLaps: 21, Avg: 56599, Gap: "4L"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			event := loadEvent(t, test.file)
			if event.Location != test.location {
				t.Errorf("location = %q, want %q", event.Location, test.location)
			}
			if event.Date != test.date {
				t.Errorf("date = %q, want %q", event.Date, test.date)
			}
			if event.RaceType != test.raceType {
				t.Errorf("race type = %q, want %q", event.RaceType, test.raceType)
			}
			if event.DriverInfo.Name != test.driver || event.DriverInfo.Position != test.position {
				t.Errorf("driver = %+v, want %s position %d", event.DriverInfo, test.driver, test.position)
			}
			for _, want := range test.rows {
				got := timeAt(t, event, want.Pos)
				if got != want {
					t.Errorf("position %d = %+v, want %+v", want.Pos, got, want)
				}
			}
		})
	}
}

func TestParseFileHighlightedRowUsesSameColumns(t *testing.T) {
	event := mustParse(t, resultMessage("Your Race Results For Daytona Milton Keynes", resultHTML(
		"Nick Stewart",
		"17th Place",
		"(2024 SODI 40min. Race)",
		`<tr><td><font color="Red">17</font></td><td><font color="Red">8</font></td><td><font color="Red">Nick Stewart</font></td><td><font color="Red">01:19:806</font></td><td><font color="Red">28</font></td><td><font color="Red">01:27:336</font></td><td><font color="Red">4L</font></td></tr>`,
	)))

	got := timeAt(t, event, 17)
	want := model.DriverTime{Pos: 17, Kart: "8", Racer: "Nick Stewart", Best: 79806, NoLaps: 28, Avg: 87336, Gap: "4L"}
	if got != want {
		t.Fatalf("highlighted row = %+v, want %+v", got, want)
	}
}

func TestParseFileErrors(t *testing.T) {
	tests := []struct {
		name    string
		message string
		reader  io.Reader
	}{
		{name: "not an email", reader: strings.NewReader("this is not an email")},
		{name: "no html", message: "Subject: Hello\r\nContent-Type: text/plain\r\n\r\nhello"},
		{name: "missing results table", message: resultMessage("Your Race Results For Daytona Milton Keynes", `<html><span id="lblName">Nick Stewart</span><span id="lblPosition">1st Place</span><span id="lblHeatType">Race</span></html>`)},
		{name: "non-numeric position", message: resultMessage("Your Race Results For Daytona Milton Keynes", resultHTML("Nick Stewart", "Place", "Race", dataRow("1", "01:15:175", "32", "01:16:539")))},
		{name: "bad lap time", message: resultMessage("Your Race Results For Daytona Milton Keynes", resultHTML("Nick Stewart", "1st Place", "Race", dataRow("1", "nope", "32", "01:16:539")))},
		{name: "bad lap count", message: resultMessage("Your Race Results For Daytona Milton Keynes", resultHTML("Nick Stewart", "1st Place", "Race", dataRow("1", "01:15:175", "many", "01:16:539")))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := test.reader
			if reader == nil {
				reader = strings.NewReader(test.message)
			}
			_, err := ParseFile(reader)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseFileUnrecognisedLocation(t *testing.T) {
	event := mustParse(t, resultMessage("Hello", resultHTML(
		"Nick Stewart",
		"1st Place",
		"Race",
		dataRow("1", "01:15:175", "32", "01:16:539"),
	)))
	if event.Location != "Unrecognised" {
		t.Fatalf("location = %q, want Unrecognised", event.Location)
	}
}

func TestParsePosition(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{in: "17th Place", want: 17},
		{in: "1st", want: 1},
		{in: "2nd", want: 2},
		{in: "3rd", want: 3},
		{in: "21st", want: 21},
		{in: "Place", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			got, err := parsePosition(test.in)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("parsePosition(%q) = %d, want %d", test.in, got, test.want)
			}
		})
	}
}

func TestConvertFromStringTime(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{in: "01:19:806", want: 79806},
		{in: "00:50:844", want: 50844},
		{in: "nope", wantErr: true},
		{in: "1:2", wantErr: true},
		{in: "a:b:c", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			got, err := convertFromStringTime(test.in)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("convertFromStringTime(%q) = %d, want %d", test.in, got, test.want)
			}
		})
	}
}

func TestFormatEventDate(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "16 Jul 2024 21:36:05 +0100", want: "16 Jul 2024"},
		{in: "5 Mar 2026 20:29:34 +0000", want: "5 Mar 2026"},
		{in: "", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			got, err := formatEventDate(test.in)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("formatEventDate(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}

func TestGetLocationFromSubject(t *testing.T) {
	tests := map[string]string{
		"Re: Your Race Results For Daytona Milton Keynes":                             "Daytona Milton Keynes",
		"Re: Your Race Results For Daytona Sandown Park Thanks for racing at Daytona": "Daytona Sandown Park",
		"Hello": "Unrecognised",
	}
	for subject, want := range tests {
		if got := getLocationFromSubject(subject); got != want {
			t.Errorf("getLocationFromSubject(%q) = %q, want %q", subject, got, want)
		}
	}
}

func loadEvent(t *testing.T, name string) *model.Event {
	t.Helper()
	file, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	event, err := ParseFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func mustParse(t *testing.T, raw string) *model.Event {
	t.Helper()
	event, err := ParseFile(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func timeAt(t *testing.T, event *model.Event, pos int) model.DriverTime {
	t.Helper()
	for _, row := range event.DriverTimes {
		if row.Pos == pos {
			return row
		}
	}
	t.Fatalf("position %d not found", pos)
	return model.DriverTime{}
}

func resultMessage(subject string, body string) string {
	return "Date: 16 Jul 2024 21:36:05 +0100\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" + body
}

func resultHTML(name string, position string, raceType string, rows string) string {
	return `<html><span id="lblName">` + name + `</span>` +
		`<span id="lblPosition">` + position + `</span>` +
		`<span id="lblHeatType">` + raceType + `</span>` +
		`<table id="dg"><tr><td>Pos</td><td>Kart</td><td>Racer</td><td>Best Lap</td><td>#Lap</td><td>Avg.</td><td>Gap</td></tr>` +
		rows + `</table></html>`
}

func dataRow(pos string, best string, laps string, avg string) string {
	return `<tr><td>` + pos + `</td><td>23</td><td>H - Craig McAl...</td><td>` + best + `</td><td>` + laps + `</td><td>` + avg + `</td><td>-</td></tr>`
}
