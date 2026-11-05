package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

func (o options) fields() []string {
	if o.Command == "gaihaku" {
		return fieldNames[3:]
	}
	switch o.Meal {
	case "morning":
		return fieldNames[:1]
	case "lunch":
		return fieldNames[1:2]
	case "dinner":
		return fieldNames[2:3]
	default:
		return fieldNames[:3]
	}
}

func (c *client) applyChange(opts options, registration *form, in *bufio.Reader, out io.Writer) error {
	before, after, err := change(registration, opts.Date, opts.fields(), opts.Operation == "add", opts.Remark)
	if err != nil {
		return err
	}
	if sameState(before, after) {
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

func showChange(out io.Writer, date string, before, after []row) {
	fmt.Fprintln(out, "変更前:")
	showDate(out, date, before)
	fmt.Fprintln(out, "変更後:")
	showDate(out, date, after)
}

func showDate(out io.Writer, date string, rows []row) {
	for _, entry := range rows {
		if entry.Date == date {
			show(out, []row{entry})
			return
		}
	}
}

// submitChange rechecks the complete snapshot and uses the latest CSRF token.
// This narrows the race window, but cannot replace server-side concurrency control.
func (c *client) submitChange(opts options, before, after []row) error {
	latest, err := c.registration()
	if err != nil {
		return err
	}
	current, err := formState(latest)
	if err != nil {
		return err
	}
	if !sameState(current, before) {
		return errors.New("確認中に状態が変わりました。再実行してください。")
	}
	if _, _, err := change(latest, opts.Date, opts.fields(), opts.Operation == "add", opts.Remark); err != nil {
		return err
	}
	target, err := resolveURL(registURL, latest.Action)
	if err != nil {
		return err
	}
	// A failed request may still have reached the server. Never retry the POST.
	if _, err := c.request(target.String(), latest.payload()); err != nil {
		return errors.New("送信結果を確認できません。自動再送しません。statusまたはブラウザで確認してください。")
	}
	actualForm, err := c.registration()
	if err != nil {
		return errors.New("送信結果を確認できません。自動再送しません。statusまたはブラウザで確認してください。")
	}
	actual, err := formState(actualForm)
	if err != nil || !sameState(actual, after) {
		return errors.New("再取得した状態が一致しません。ブラウザで登録結果を確認してください。自動再送しません。")
	}
	return nil
}
