package cli

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf16"
)

var applicationFields = []string{"kesshokuMorningDate", "kesshokuLunchDate", "kesshokuDinnerDate", "gaihakuDate"}

type fieldState struct{ Checked, Enabled bool }
type scheduleRow struct {
	Date, Remark string
	Fields       map[string]fieldState
}

func scheduleFromForm(f *htmlForm) ([]scheduleRow, error) {
	var rows []scheduleRow
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
			rows = append(rows, scheduleRow{Date: d, Fields: map[string]fieldState{}})
			continue
		}
		if name == "remarkList" {
			remarks++
		}
		if len(rows) == 0 {
			continue
		}
		r := &rows[len(rows)-1]
		if isApplicationField(name) {
			if a["value"] != r.Date || strings.ToLower(a["type"]) != "checkbox" {
				return nil, errors.New("想定外の日付・入力形式です。")
			}
			r.Fields[name] = fieldState{hasAttribute(a, "checked"), !hasAttribute(a, "disabled")}
		} else if name == "remarkList" {
			r.Remark = a["value"]
		}
	}
	if len(rows) == 0 || remarks != len(rows) {
		return nil, errors.New("日付と備考の対応を確認できません。")
	}
	return rows, nil
}
func isApplicationField(s string) bool {
	for _, f := range applicationFields {
		if s == f {
			return true
		}
	}
	return false
}
func applyFormChange(f *htmlForm, date string, fields []string, enabled bool, remark *string) ([]scheduleRow, []scheduleRow, error) {
	before, err := scheduleFromForm(f)
	if err != nil {
		return nil, nil, err
	}
	var target *scheduleRow
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
			if strings.ToLower(a["type"]) != "text" || hasAttribute(a, "disabled") {
				return nil, nil, errors.New("備考は変更できません。")
			}
			max := 100
			if hasAttribute(a, "maxlength") {
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
	after, err := scheduleFromForm(f)
	return before, after, err
}
func sameSchedule(a, b []scheduleRow) bool { return reflect.DeepEqual(a, b) }
