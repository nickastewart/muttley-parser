package parser

import (
	"fmt"
	"io"
	"net/mail"

	"github.com/nickastewart/muttley-parser/model"
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
