package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

func (o commandOptions) fields() []string {
	if o.Command == "gaihaku" {
		return applicationFields[3:]
	}
	switch o.Meal {
	case "morning":
		return applicationFields[:1]
	case "lunch":
		return applicationFields[1:2]
	case "dinner":
		return applicationFields[2:3]
	default:
		return applicationFields[:3]
	}
}

func (c *siteClient) applyChange(opts commandOptions, registrationForm *htmlForm, in *bufio.Reader, out io.Writer) error {
	before, after, err := applyFormChange(registrationForm, opts.Date, opts.fields(), opts.Operation == "add", opts.Remark)
	if err != nil {
		return err
	}
	if sameSchedule(before, after) {
		fmt.Fprintln(out, "変更はありません。")
		return nil
	}
	showChange(out, opts.Date, before, after)
	if !opts.Apply {
		fmt.Fprintln(out, "表示のみ。登録するには同じコマンドに --apply を付けてください。")
		return nil
	}
	if !opts.Yes {
		confirmed, err := confirmChange(in, out)
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
	}
	if err := c.submitChange(opts, before, after); err != nil {
		return err
	}
	fmt.Fprintln(out, "登録後の状態を再取得し、一致を確認しました。")
	return nil
}

func confirmChange(in *bufio.Reader, out io.Writer) (bool, error) {
	fmt.Fprint(out, "この変更を登録しますか？ [y/N]: ")
	answer, err := readLine(in)
	return strings.EqualFold(answer, "y"), err
}

func showChange(out io.Writer, date string, before, after []scheduleRow) {
	fmt.Fprintln(out, "変更前:")
	showDate(out, date, before)
	fmt.Fprintln(out, "変更後:")
	showDate(out, date, after)
}

func showDate(out io.Writer, date string, rows []scheduleRow) {
	for _, entry := range rows {
		if entry.Date == date {
			renderSchedule(out, []scheduleRow{entry})
			return
		}
	}
}

// submitChange rechecks the complete snapshot and uses the latest CSRF token.
// This narrows the race window, but cannot replace server-side concurrency control.
func (c *siteClient) submitChange(opts commandOptions, before, after []scheduleRow) error {
	latest, err := c.fetchRegistration()
	if err != nil {
		return err
	}
	current, err := scheduleFromForm(latest)
	if err != nil {
		return err
	}
	if !sameSchedule(current, before) {
		return errors.New("確認中に状態が変わりました。再実行してください。")
	}
	if _, _, err := applyFormChange(latest, opts.Date, opts.fields(), opts.Operation == "add", opts.Remark); err != nil {
		return err
	}
	target, err := resolveURL(registrationURL, latest.Action)
	if err != nil {
		return err
	}
	// A failed request may still have reached the server. Never retry the POST.
	if _, err := c.request(target.String(), latest.payload()); err != nil {
		return errors.New("送信結果を確認できません。自動再送しません。statusまたはブラウザで確認してください。")
	}
	actualForm, err := c.fetchRegistration()
	if err != nil {
		return errors.New("送信結果を確認できません。自動再送しません。statusまたはブラウザで確認してください。")
	}
	actual, err := scheduleFromForm(actualForm)
	if err != nil || !sameSchedule(actual, after) {
		return errors.New("再取得した状態が一致しません。ブラウザで登録結果を確認してください。自動再送しません。")
	}
	return nil
}
