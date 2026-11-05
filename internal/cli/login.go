package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// OTP is submitted only when the server supplies the expected confirmation form.
// Neither the code nor the response HTML is persisted.
func (c *siteClient) confirmOTP(source string, in *bufio.Reader, out io.Writer) error {
	found := false
	if err := scanHTMLTags(source, func(tag string, a htmlAttributes) error {
		if tag == "form" && a["id"] == "tlogin" {
			found = true
		}
		return nil
	}); err != nil {
		return err
	}
	if !found {
		if _, err := parseForm(source, "login"); err == nil {
			return errors.New("ログインが拒否されました。学籍番号とパスワードを確認してください。")
		}
		return nil
	}
	f, err := parseForm(source, "tlogin")
	if err != nil {
		return err
	}
	var codeField htmlAttributes
	for _, a := range f.Inputs {
		if a["name"] == "authCode" && !hasAttribute(a, "disabled") {
			if codeField != nil {
				return errors.New("OTP入力欄が複数あります。")
			}
			codeField = a
		}
	}
	if codeField == nil {
		return errors.New("OTP入力欄が見つかりません。画面変更の可能性があります。")
	}
	target, err := resolveURL(loginURL, f.Action)
	if err != nil {
		return err
	}
	fmt.Fprint(out, "メールに届いた認証コード（OTP）: ")
	code, err := readLine(in)
	if err != nil {
		return errors.New("認証コードを読み取れません。loginを再実行してください。")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return errors.New("認証コードが空のため、ログインを中止しました。")
	}
	codeField["value"] = code
	s, err := c.request(target.String(), f.payload())
	if err != nil {
		return err
	}
	for _, id := range []string{"tlogin", "login"} {
		if _, err := parseForm(s, id); err == nil {
			return errors.New("OTP認証が完了しませんでした。コードの誤り・期限切れの可能性があります。loginを再実行してください。")
		}
	}
	return nil
}

// login completes both password and optional email-code authentication before
// checking that the protected registration form is accessible.
func (c *siteClient) login(in *bufio.Reader, out io.Writer, secret func() (string, error)) error {
	s, err := c.request(loginURL, nil)
	if err != nil {
		return err
	}
	f, err := parseForm(s, "login")
	if err != nil {
		return err
	}
	userField, passwordField := false, false
	for _, a := range f.Inputs {
		if a["name"] == "gakusekiNo" && !hasAttribute(a, "disabled") {
			userField = true
		}
		if a["name"] == "loginPass" && !hasAttribute(a, "disabled") {
			passwordField = true
		}
	}
	if !userField || !passwordField {
		return errors.New("ログイン入力欄が見つかりません。画面変更の可能性があります。")
	}
	user, password, err := loginCredentials(".env", in, out, secret)
	if err != nil {
		return err
	}
	for _, a := range f.Inputs {
		switch a["name"] {
		case "gakusekiNo":
			a["value"] = user
		case "loginPass":
			a["value"] = password
		case "rememberMe":
			delete(a, "checked")
		}
	}
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
