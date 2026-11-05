package cli

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	requestTimeout   = 30 * time.Second
	maxRedirects     = 10
	maxResponseBytes = 8 * 1024 * 1024
)

type siteClient struct {
	HTTP *http.Client
	Jar  *persistentJar
	Path string
}

func newClient() (*siteClient, error) {
	path, err := cookiePath()
	if err != nil {
		return nil, err
	}
	jar, err := loadCookieJar(path)
	if err != nil {
		return nil, err
	}
	return &siteClient{
		Jar:  jar,
		Path: path,
		HTTP: &http.Client{Jar: jar, Timeout: requestTimeout, CheckRedirect: checkRedirect},
	}, nil
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return errors.New("リダイレクトが多すぎます。")
	}
	// 307/308 can replay a form POST. An ambiguous submission must be checked,
	// never sent again automatically.
	if req.Method == http.MethodPost {
		return errors.New("リダイレクトによるPOSTの再送を拒否しました。")
	}
	_, err := validateURL(req.URL.String())
	return err
}

func (c *siteClient) request(target string, data []formValue) (string, error) {
	u, err := validateURL(target)
	if err != nil {
		return "", err
	}
	method := http.MethodGet
	var body io.Reader
	if data != nil {
		method = http.MethodPost
		body = strings.NewReader(encodePairs(data))
	}
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return "", errors.New("リクエストを作成できません。")
	}
	req.Header.Set("User-Agent", "kesshock/1.0")
	req.Header.Set("Accept", "text/html")
	if data != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", baseURL)
		req.Header.Set("Referer", loginURL)
		if strings.Contains(u.Path, "/regist/") {
			req.Header.Set("Referer", registrationURL)
		}
	}
	response, err := c.HTTP.Do(req)
	if err != nil {
		return "", errors.New("通信に失敗しました。送信した場合はstatusまたはブラウザで状態を確認してください。")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("サーバーがHTTP %dを返しました。", response.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return "", errors.New("応答を読み取れません。")
	}
	if len(b) > maxResponseBytes {
		return "", errors.New("応答サイズが大きすぎます。")
	}
	if err = c.save(); err != nil {
		return "", err
	}
	return string(b), nil
}
func (c *siteClient) fetchRegistration() (*htmlForm, error) {
	s, err := c.request(registrationURL, nil)
	if err != nil {
		return nil, err
	}
	return parseForm(s, "regist")
}
