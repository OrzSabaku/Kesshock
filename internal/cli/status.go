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
	registration, err := parseForm(string(source), "regist")
	if err != nil {
		return err
	}
	return showStatus(registration, out)
}

func showStatus(registration *form, out io.Writer) error {
	rows, err := formState(registration)
	if err != nil {
		return err
	}
	show(out, rows)
	return nil
}
