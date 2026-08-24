package favicon

import (
	"encoding/base64"
	"errors"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	MaxRaw        = 16 << 10
	maxEmojiRunes = 16
)

var (
	ErrTooLong  = errors.New("too long")
	ErrInvalid  = errors.New("not an emoji or image data URI")
	hrefAttrRe  = regexp.MustCompile(`(?i)href\s*=\s*["']([^"']+)["']`)
	dataImageRe = regexp.MustCompile(`(?i)^data:image/(svg\+xml|png|x-icon|vnd\.microsoft\.icon|webp|gif|jpeg|jpg);base64,[A-Za-z0-9+/=]+$`)
	base64Re    = regexp.MustCompile(`^[A-Za-z0-9+/]+=*$`)
)

func Href(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > MaxRaw {
		return "", ErrTooLong
	}
	if strings.Contains(strings.ToLower(raw), "<link") {
		if href := extractHref(raw); href != "" {
			raw = href
		}
	}
	if strings.HasPrefix(strings.ToLower(raw), "data:") {
		return sanitizeDataURI(raw)
	}
	compact := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' {
			return -1
		}
		return r
	}, raw)
	if len(compact) >= 24 && isBase64(compact) {
		return "data:image/svg+xml;base64," + compact, nil
	}
	return emojiDataURI(raw)
}

func LinkTag(raw string) string {
	href, err := Href(raw)
	if err != nil || href == "" {
		return ""
	}
	return `<link rel="icon" href="` + html.EscapeString(href) + `">`
}

func extractHref(raw string) string {
	m := hrefAttrRe.FindStringSubmatch(raw)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func sanitizeDataURI(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !dataImageRe.MatchString(raw) {
		return "", ErrInvalid
	}
	return raw, nil
}

func isBase64(s string) bool {
	if len(s)%4 != 0 || !base64Re.MatchString(s) {
		return false
	}
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

func emojiDataURI(text string) (string, error) {
	if utf8.RuneCountInString(text) > maxEmojiRunes {
		return "", ErrInvalid
	}
	for _, r := range text {
		if r < 0x20 || r == 0x7f || r == '<' || r == '>' || r == '"' || r == '\'' {
			return "", ErrInvalid
		}
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text y=".9em" font-size="90">` +
		html.EscapeString(text) + `</text></svg>`
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg)), nil
}
