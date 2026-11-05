package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type htmlAttributes map[string]string
type htmlForm struct {
	Action string
	Inputs []htmlAttributes
}
type formValue struct{ Name, Value string }

func hasAttribute(a htmlAttributes, key string) bool { _, ok := a[key]; return ok }

func parseForm(source, wanted string) (*htmlForm, error) {
	f := &htmlForm{}
	active, found := false, false
	err := scanHTMLTags(source, func(tag string, a htmlAttributes) error {
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

// payload preserves document order because the server pairs repeated date and remark fields by position.
func (f *htmlForm) payload() []formValue {
	var result []formValue
	for _, a := range f.Inputs {
		typ := strings.ToLower(a["type"])
		if hasAttribute(a, "disabled") || a["name"] == "" || typ == "button" || typ == "reset" || typ == "file" {
			continue
		}
		if (typ == "checkbox" || typ == "radio") && !hasAttribute(a, "checked") {
			continue
		}
		value := a["value"]
		if !hasAttribute(a, "value") && (typ == "checkbox" || typ == "radio") {
			value = "on"
		}
		result = append(result, formValue{a["name"], value})
	}
	return result
}
func encodePairs(pairs []formValue) string {
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, url.QueryEscape(p.Name)+"="+url.QueryEscape(p.Value))
	}
	return strings.Join(parts, "&")
}
