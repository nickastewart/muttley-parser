package parser

import (
	"fmt"
	"io"
	"os"
	"testing"
)

func TestParserLocationMiltonKeynes(t *testing.T) {

	file, err := os.Open("2024-07-16-Milton-Keynes.eml")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	reader := io.Reader(file)
	event, err := ParseFile("Daytona Milton Keynes", reader)

	if event.Location != "Daytona Milton Keynes" || err != nil {
		t.Errorf(`ParseFile("2024-07-16-Milton-Keynes.eml") = %q, %v, want match for %#q, nil`, event.Location, err, "Daytona Milton Keynes")
	}

	if event.Date != "16 Jul 2024" {
		t.Errorf(`ParseFile("2024-07-16-Milton-Keynes.eml") = %q, %v, want match for %#q, nil`, event.Date, err, "16 Jul 2024")
	}
}

func TestParserSandown(t *testing.T) {

	file, err := os.Open("2026-03-05-sandown.eml")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	reader := io.Reader(file)
	event, err := ParseFile("Daytona Sandown Park", reader)

	if err != nil {
		t.Errorf(`ParseFile("2026-03-05-sandown.eml"), error was not nil %v`, err)
	}

	if event.Location != "Daytona Sandown Park" {
		t.Errorf(`ParseFile("2026-03-05-sandown.eml") = %q, %v, want match for %#q, nil`, event.Location, err, "Daytona Sandown Park")
	}

	if event.Date != "5 Mar 2026" {
		t.Errorf(`ParseFile("2026-03-05-sandown.eml") = %q, %v, want match for %#q, nil`, event.Date, err, "5 Mar 2026")
	}

}
