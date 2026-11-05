package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// OTP is submitted only when the server supplies the expected confirmation form.
// Neither the code nor the response HTML is persisted.
func (c *client) confirmOTP(source string, in *bufio.Reader, out io.Writer) error {
	found := false
	if err := tags(source, func(tag string, a input) error {
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
	var codeField input
	for _, a := range f.Inputs {
		if a["name"] == "authCode" && !has(a, "disabled") {
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
