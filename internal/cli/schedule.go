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

// scheduleFromForm interprets controls in document order. A missing checkbox
// means the day is unavailable, not that the student has chosen to eat.
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
			r.Fields[name] = fieldState{Checked: hasAttribute(a, "checked"), Enabled: !hasAttribute(a, "disabled")}
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

// applyFormChange validates every requested edit before changing the form.
// Unrelated controls, including hidden tokens and other dates, remain untouched.
func applyFormChange(form *htmlForm, date string, fields []string, checked bool, remark *string) ([]scheduleRow, []scheduleRow, error) {
	before, err := scheduleFromForm(form)
	if err != nil {
		return nil, nil, err
	}
	target := findScheduleRow(before, date)
	if target == nil {
		return nil, nil, errors.New("対象日は現在の画面にありません。")
	}
	for _, name := range fields {
		state, exists := target.Fields[name]
		if !exists || !state.Enabled {
			return nil, nil, fmt.Errorf("%s の %s は変更できません。", date, name)
		}
	}
	inputs := form.inputsForDate(date)
	if remark != nil {
		if err := validateRemarkInputs(inputs, *remark); err != nil {
			return nil, nil, err
		}
	}
	for _, attributes := range inputs {
		if containsField(fields, attributes["name"]) {
			if checked {
				attributes["checked"] = ""
			} else {
				delete(attributes, "checked")
			}
		}
		if attributes["name"] == "remarkList" && remark != nil {
			attributes["value"] = *remark
		}
	}
	after, err := scheduleFromForm(form)
	return before, after, err
}

func findScheduleRow(rows []scheduleRow, date string) *scheduleRow {
	for i := range rows {
		if rows[i].Date == date {
			return &rows[i]
		}
	}
	return nil
}

// inputsForDate follows the site's positional grouping rather than assuming
// unique input IDs; the server uses repeated names for every day.
func (form *htmlForm) inputsForDate(date string) []htmlAttributes {
	var inputs []htmlAttributes
	currentDate := ""
	for _, attributes := range form.Inputs {
		if attributes["name"] == "dateStrList" {
			currentDate = attributes["value"]
		}
		if currentDate == date {
			inputs = append(inputs, attributes)
		}
	}
	return inputs
}

func containsField(fields []string, name string) bool {
	for _, field := range fields {
		if field == name {
			return true
		}
	}
	return false
}

func validateRemarkInputs(inputs []htmlAttributes, remark string) error {
	found := false
	for _, attributes := range inputs {
		if attributes["name"] != "remarkList" {
			continue
		}
		found = true
		if err := validateRemark(attributes, remark); err != nil {
			return err
		}
	}
	if !found {
		return errors.New("備考欄が見つかりません。")
	}
	return nil
}

func validateRemark(attributes htmlAttributes, remark string) error {
	if !strings.EqualFold(attributes["type"], "text") || hasAttribute(attributes, "disabled") {
		return errors.New("備考は変更できません。")
	}
	maxLength := 100
	if hasAttribute(attributes, "maxlength") {
		parsed, err := strconv.Atoi(attributes["maxlength"])
		if err != nil || parsed < 0 {
			return errors.New("備考の文字数制限が不正です。")
		}
		maxLength = parsed
	}
	// HTML maxlength counts UTF-16 code units, so emoji can count as two.
	if len(utf16.Encode([]rune(remark))) > maxLength {
		return errors.New("備考が長すぎます。")
	}
	return nil
}

func sameSchedule(a, b []scheduleRow) bool { return reflect.DeepEqual(a, b) }
