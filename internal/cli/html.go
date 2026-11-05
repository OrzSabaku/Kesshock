package cli

import (
	"html"
	"strings"
	"unicode"
)

// scanHTMLTags handles only the input-based forms used by this site. It is not
// a general HTML parser: comments and raw-text content must never become controls.
func scanHTMLTags(source string, visit func(string, htmlAttributes) error) error {
	for len(source) > 0 {
		start := strings.IndexByte(source, '<')
		if start < 0 {
			return nil
		}
		source = source[start:]
		if strings.HasPrefix(source, "<!--") {
			end := strings.Index(source, "-->")
			if end < 0 {
				return nil
			}
			source = source[end+3:]
			continue
		}
		end := tagEnd(source)
		if end < 0 {
			return nil
		}
		name, attributes := parseTag(source[1:end])
		source = source[end+1:]
		if err := visit(name, attributes); err != nil {
			return err
		}
		if isRawTextTag(name) {
			close := strings.Index(strings.ToLower(source), "</"+name)
			if close < 0 {
				return nil
			}
			source = source[close:]
		}
	}
	return nil
}

// tagEnd ignores greater-than characters inside quoted attribute values.
func tagEnd(source string) int {
	var quote byte
	for i := 1; i < len(source); i++ {
		current := source[i]
		if quote != 0 {
			if current == quote {
				quote = 0
			}
			continue
		}
		switch current {
		case '\'', '"':
			quote = current
		case '>':
			return i
		}
	}
	return -1
}

func parseTag(raw string) (string, htmlAttributes) {
	raw = strings.TrimSpace(raw)
	nameEnd := strings.IndexFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || r == '>' })
	if nameEnd < 0 {
		nameEnd = len(raw)
	}
	name := strings.ToLower(raw[:nameEnd])
	rest := raw[nameEnd:]
	attributes := htmlAttributes{}
	for len(rest) > 0 {
		rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
		if rest == "" || rest == "/" {
			break
		}
		keyEnd := 0
		for keyEnd < len(rest) && !strings.ContainsRune(" \t\r\n\f=", rune(rest[keyEnd])) {
			keyEnd++
		}
		if keyEnd == 0 {
			rest = rest[1:]
			continue
		}
		key := strings.ToLower(rest[:keyEnd])
		rest = strings.TrimLeftFunc(rest[keyEnd:], unicode.IsSpace)
		value := ""
		if strings.HasPrefix(rest, "=") {
			value, rest = consumeAttributeValue(rest[1:])
		}
		// Browsers use the first occurrence of a duplicate attribute.
		if !hasAttribute(attributes, key) {
			attributes[key] = html.UnescapeString(value)
		}
	}
	return name, attributes
}

func consumeAttributeValue(source string) (value, rest string) {
	source = strings.TrimLeftFunc(source, unicode.IsSpace)
	if len(source) > 0 && (source[0] == '\'' || source[0] == '"') {
		quote := source[0]
		source = source[1:]
		end := strings.IndexByte(source, quote)
		if end < 0 {
			return source, ""
		}
		return source[:end], source[end+1:]
	}
	end := strings.IndexFunc(source, unicode.IsSpace)
	if end < 0 {
		return source, ""
	}
	return source[:end], source[end:]
}

func isRawTextTag(name string) bool {
	switch name {
	case "script", "style", "textarea", "title":
		return true
	default:
		return false
	}
}
