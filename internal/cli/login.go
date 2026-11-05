package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

// login completes both password and optional email-code authentication before
// checking that the protected registration form is accessible.
func (c *siteClient) login(in *bufio.Reader, out io.Writer, readSecret func() (string, error)) error {
	s, err := c.request(loginURL, nil)
	if err != nil {
		return err
	}
	f, err := parseForm(s, "login")
	if err != nil {
		return err
	}
	if !f.hasEnabledInput("gakusekiNo") || !f.hasEnabledInput("loginPass") {
		return errors.New("ログイン入力欄が見つかりません。画面変更の可能性があります。")
	}
	user, password, err := loginCredentials(".env", in, out, readSecret)
	if err != nil {
		return err
	}
	fillLoginForm(f, user, password)
	target, err := resolveURL(loginURL, f.Action)
	if err != nil {
		return err
	}
	s, err = c.request(target.String(), f.payload())
	if err != nil {
		return err
	}
	if err = c.confirmOTP(s, in, out); err != nil {
		return err
	}
	if _, err = c.fetchRegistration(); err != nil {
		return err
	}
	fmt.Fprintln(out, "ログインを確認しました。")
	return nil
}

func (f *htmlForm) hasEnabledInput(name string) bool {
	for _, attributes := range f.Inputs {
		if attributes["name"] == name && !hasAttribute(attributes, "disabled") {
			return true
		}
	}
	return false
}

func fillLoginForm(form *htmlForm, studentID, password string) {
	for _, attributes := range form.Inputs {
		switch attributes["name"] {
		case "gakusekiNo":
			attributes["value"] = studentID
		case "loginPass":
			attributes["value"] = password
		case "rememberMe":
			// Long-lived login remains opt-in; storing cookies alone does not enable it.
			delete(attributes, "checked")
		}
	}
}
