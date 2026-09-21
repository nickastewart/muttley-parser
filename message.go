package parser

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
)

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
