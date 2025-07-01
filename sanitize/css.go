package sanitize

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

func CSSSanitizeAndExtractUrls(raw string, urls map[string]struct{}, sanitizeURL func(string) string) (string, error) {
	var sb strings.Builder

	sb.Grow(len(raw))

	input := parse.NewInput(bytes.NewBufferString(raw))
	lexer := css.NewLexer(input)

	last := 0

	for {
		from := input.Offset()
		token, _ := lexer.Next()
		to := input.Offset()

		if token == css.ErrorToken {
			if errors.Is(lexer.Err(), io.EOF) {
				sb.WriteString(raw[last:from])
				break
			}

			return "", fmt.Errorf("lexing css failed: %w", lexer.Err())
		}

		if token == css.URLToken || token == css.BadURLToken {
			sb.WriteString(raw[last:from])
			updated := cssSanitizeURL(raw[from:to], urls, sanitizeURL)
			sb.WriteString(updated)

			last = to
		}
	}

	return sb.String(), nil
}

//nolint:gochecknoglobals
var cssSingleQuotedStringReplacer = strings.NewReplacer(
	`\`, `\\`,
	`'`, `\'`,
	"\r", `\r`,
	"\n", `\n`,
	"\f", `\f`,
	"\t", `\t`,
	"\x00", `\0`,
)

//nolint:gochecknoglobals
var cssDoubleQuotedStringReplacer = strings.NewReplacer(
	`\`, `\\`,
	`"`, `\"`,
	"\r", `\r`,
	"\n", `\n`,
	"\f", `\f`,
	"\t", `\t`,
	"\x00", `\0`,
)

//nolint:gochecknoglobals
var cssUnquotedURLReplacer = strings.NewReplacer(
	`\`, `\\`,
	`"`, `\"`,
	`'`, `\'`,
	`(`, `\(`,
	`)`, `\)`,
	" ", `\ `,
	"\r", `\r`,
	"\n", `\n`,
	"\f", `\f`,
	"\t", `\t`,
	"\x00", `\0`,
)

func cssSanitizeURL(raw string, urls map[string]struct{}, sanitizeURL func(string) string) string {
	if len(raw) < len("url()") || !strings.EqualFold(raw[:len("url(")], "url(") || raw[len(raw)-1] != ')' {
		return raw
	}

	raw = raw[len("url(") : len(raw)-1]
	raw = strings.TrimSpace(raw)

	if raw == "" {
		return "url()"
	}

	var q string
	var replacer *strings.Replacer

	switch raw[0] {
	case '\'':
		q = "'"
		replacer = cssSingleQuotedStringReplacer

		end := strings.LastIndexByte(raw[1:], '\'')
		if end == -1 {
			raw = raw[1:]
		} else {
			raw = raw[1 : end+1]
		}

		raw = strings.ReplaceAll(raw, "\\'", "'")
	case '"':
		q = "\""
		replacer = cssDoubleQuotedStringReplacer

		end := strings.LastIndexByte(raw[1:], '"')
		if end == -1 {
			raw = raw[1:]
		} else {
			raw = raw[1 : end+1]
		}

		raw = strings.ReplaceAll(raw, "\\\"", "\"")
	default:
		q = ""
		replacer = cssUnquotedURLReplacer
	}

	raw = cssUnescape(raw)

	sanitized := sanitizeURL(raw)
	urls[sanitized] = struct{}{}

	sanitized = replacer.Replace(sanitized)
	sanitized = "url(" + q + sanitized + q + ")"

	return sanitized
}

var cssUnescapeRe = regexp.MustCompile(`\\([0-9a-fA-F]{1,6}) ?`)

//nolint:cyclop
func cssUnescape(input string) string {
	v := cssUnescapeRe.ReplaceAllStringFunc(input, func(match string) string {
		hex := strings.TrimSpace(match[1:])

		code, err := strconv.ParseInt(hex, 16, 32)
		if err != nil {
			return match
		}

		return string(rune(code))
	})

	var sb strings.Builder
	escaped := false

	for _, r := range v {
		if !escaped {
			if r == '\\' {
				escaped = true
				continue
			}

			sb.WriteRune(r)

			continue
		}

		switch r {
		case '\r':
			continue
		case '\n', '\f':
			escaped = false
			continue
		case 'n':
			sb.WriteRune('\n')
		case 'r':
			sb.WriteRune('\r')
		case 't':
			sb.WriteRune('\t')
		case 'f':
			sb.WriteRune('\f')
		case '\\':
			sb.WriteRune('\\')
		default:
			sb.WriteRune(r)
		}

		escaped = false
	}

	if escaped {
		sb.WriteByte('\\')
	}

	return sb.String()
}
