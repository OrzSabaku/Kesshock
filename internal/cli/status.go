package cli

import (
	"errors"
	"io"
	"os"
)

func showSavedStatus(path string, out io.Writer) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return errors.New("HTMLファイルを読めません。")
	}
	registrationForm, err := parseForm(string(source), "regist")
	if err != nil {
		return err
	}
	return showStatus(registrationForm, out)
}

func showStatus(registrationForm *htmlForm, out io.Writer) error {
	rows, err := scheduleFromForm(registrationForm)
	if err != nil {
		return err
	}
	renderSchedule(out, rows)
	return nil
}
