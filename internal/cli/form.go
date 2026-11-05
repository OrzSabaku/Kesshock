package cli

import (
	"errors"
	"fmt"
	"html"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
)

type input map[string]string
type form struct {
	Action string
	Inputs []input
}
type pair struct{ Name, Value string }

var fieldNames = []string{"kesshokuMorningDate", "kesshokuLunchDate", "kesshokuDinnerDate", "gaihakuDate"}

func has(a input, key string) bool { _, ok := a[key]; return ok }

// tags scans HTML tags without treating > inside quoted attributes as a delimiter.
// Comments and raw-text elements cannot introduce form controls.
func tags(source string, visit func(string, input) error) error {
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
		quote := byte(0)
		end := 1
		for ; end < len(source); end++ {
			c := source[end]
			if quote != 0 {
				if c == quote {
					quote = 0
				}
			} else if c == '\'' || c == '"' {
				quote = c
			} else if c == '>' {
				break
			}
		}
		if end == len(source) {
			return nil
		}
		raw := strings.TrimSpace(source[1:end])
		source = source[end+1:]
		nameEnd := strings.IndexFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || r == '>' })
		if nameEnd < 0 {
			nameEnd = len(raw)
		}
		name := strings.ToLower(raw[:nameEnd])
		rest := raw[nameEnd:]
		attrs := input{}
		for len(rest) > 0 {
			rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
			if rest == "" || rest == "/" {
				break
			}
			n := 0
			for n < len(rest) && !strings.ContainsRune(" \t\r\n\f=", rune(rest[n])) {
				n++
			}
			if n == 0 {
				rest = rest[1:]
				continue
			}
			key := strings.ToLower(rest[:n])
			rest = strings.TrimLeftFunc(rest[n:], unicode.IsSpace)
			value := ""
			if strings.HasPrefix(rest, "=") {
				rest = strings.TrimLeftFunc(rest[1:], unicode.IsSpace)
				if len(rest) > 0 && (rest[0] == '\'' || rest[0] == '"') {
					q := rest[0]
					rest = rest[1:]
					n = strings.IndexByte(rest, q)
					if n < 0 {
						n = len(rest)
					}
					value = rest[:n]
					rest = rest[n:]
					if len(rest) > 0 {
						rest = rest[1:]
					}
				} else {
					n = strings.IndexFunc(rest, unicode.IsSpace)
					if n < 0 {
						n = len(rest)
					}
					value = rest[:n]
					rest = rest[n:]
				}
			}
			if !has(attrs, key) {
				attrs[key] = html.UnescapeString(value)
			}
		}
		if err := visit(name, attrs); err != nil {
			return err
		}
		if name == "script" || name == "style" || name == "textarea" || name == "title" {
			close := strings.Index(strings.ToLower(source), "</"+name)
			if close < 0 {
				return nil
			}
			source = source[close:]
		}
	}
	return nil
}
func parseForm(source, wanted string) (*form, error) {
	f := &form{}
	active, found := false, false
	err := tags(source, func(tag string, a input) error {
		switch tag {
		case "form":
			active = a["id"] == wanted
			if active {
				if found {
					return errors.New("対象フォームが複数あります。")
				}
				found = true
				f.Action = a["action"]
				if strings.ToLower(a["method"]) != "post" {
					return errors.New("想定外のフォーム送信方式です。")
				}
			}
		case "/form":
			active = false
		case "input":
			if active {
				f.Inputs = append(f.Inputs, a)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("%sフォームがありません。ログイン切れ・画面変更の可能性があります。", wanted)
	}
	return f, nil
}
func (f *form) payload() []pair {
	var result []pair
	for _, a := range f.Inputs {
		typ := strings.ToLower(a["type"])
		if has(a, "disabled") || a["name"] == "" || typ == "button" || typ == "reset" || typ == "file" {
			continue
		}
		if (typ == "checkbox" || typ == "radio") && !has(a, "checked") {
			continue
		}
		value := a["value"]
		if !has(a, "value") && (typ == "checkbox" || typ == "radio") {
			value = "on"
		}
		result = append(result, pair{a["name"], value})
	}
	return result
}
func encodePairs(pairs []pair) string {
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, url.QueryEscape(p.Name)+"="+url.QueryEscape(p.Value))
	}
	return strings.Join(parts, "&")
}

type check struct{ Checked, Enabled bool }
type row struct {
	Date, Remark string
	Fields       map[string]check
}

func formState(f *form) ([]row, error) {
	var rows []row
	seen := map[string]bool{}
	remarks := 0
	for _, a := range f.Inputs {
		name := a["name"]
		if name == "dateStrList" {
			d := a["value"]
			if d == "" || seen[d] {
				return nil, errors.New("日付が空または重複しています。")
			}
			seen[d] = true
			rows = append(rows, row{Date: d, Fields: map[string]check{}})
			continue
		}
		if name == "remarkList" {
			remarks++
		}
		if len(rows) == 0 {
			continue
		}
		r := &rows[len(rows)-1]
		if isField(name) {
			if a["value"] != r.Date || strings.ToLower(a["type"]) != "checkbox" {
				return nil, errors.New("想定外の日付・入力形式です。")
			}
			r.Fields[name] = check{has(a, "checked"), !has(a, "disabled")}
		} else if name == "remarkList" {
			r.Remark = a["value"]
		}
	}
	if len(rows) == 0 || remarks != len(rows) {
		return nil, errors.New("日付と備考の対応を確認できません。")
	}
	return rows, nil
}
func isField(s string) bool {
	for _, f := range fieldNames {
		if s == f {
			return true
		}
	}
	return false
}
func change(f *form, date string, fields []string, enabled bool, remark *string) ([]row, []row, error) {
	before, err := formState(f)
	if err != nil {
		return nil, nil, err
	}
	var target *row
	for i := range before {
		if before[i].Date == date {
			target = &before[i]
			break
		}
	}
	if target == nil {
		return nil, nil, errors.New("対象日は現在の画面にありません。")
	}
	for _, field := range fields {
		v, ok := target.Fields[field]
		if !ok || !v.Enabled {
			return nil, nil, fmt.Errorf("%s の %s は変更できません。", date, field)
		}
	}
	current := ""
	foundRemark := false
	for _, a := range f.Inputs {
		if a["name"] == "dateStrList" {
			current = a["value"]
		}
		if current != date {
			continue
		}
		for _, field := range fields {
			if a["name"] == field {
				if enabled {
					a["checked"] = ""
				} else {
					delete(a, "checked")
				}
			}
		}
		if a["name"] == "remarkList" && remark != nil {
			if strings.ToLower(a["type"]) != "text" || has(a, "disabled") {
				return nil, nil, errors.New("備考は変更できません。")
			}
			max := 100
			if has(a, "maxlength") {
				max, err = strconv.Atoi(a["maxlength"])
				if err != nil || max < 0 {
					return nil, nil, errors.New("備考の文字数制限が不正です。")
				}
			}
			if len(utf16.Encode([]rune(*remark))) > max {
				return nil, nil, errors.New("備考が長すぎます。")
			}
			a["value"] = *remark
			foundRemark = true
		}
	}
	if remark != nil && !foundRemark {
		return nil, nil, errors.New("備考欄が見つかりません。")
	}
	after, err := formState(f)
	return before, after, err
}
func sameState(a, b []row) bool { return reflect.DeepEqual(a, b) }
