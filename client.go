package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const baseURL = "https://gaihaku.mogsmile.com"
const loginURL = baseURL + "/takuma/student/login!init.action"
const registURL = baseURL + "/takuma/student/regist/regist.action"

func checkedURL(target string) (*url.URL, error) { return resolveURL(baseURL, target) }
func resolveURL(base, target string) (*url.URL, error) {
	b, err := url.Parse(base)
	if err != nil {
		return nil, errors.New("送信先URLが不正です。")
	}
	u, err := url.Parse(target)
	if err != nil {
		return nil, errors.New("送信先URLが不正です。")
	}
	u = b.ResolveReference(u)
	if u.Scheme != "https" || u.Host != "gaihaku.mogsmile.com" || u.User != nil || !strings.HasPrefix(u.Path, "/takuma/student/") {
		return nil, errors.New("想定外の送信先を拒否しました。")
	}
	return u, nil
}

type storedCookie struct {
	Origin string
	Cookie *http.Cookie
}

// Keep the original scope and expiry of cookies, including session cookies.
// cookiejar handles matching for requests; Records retains attributes for disk storage.
type persistentJar struct {
	*cookiejar.Jar
	Records []storedCookie
}

func (j *persistentJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.Jar.SetCookies(u, cookies)
	for _, c := range cookies {
		copyCookie := *c
		if copyCookie.Path == "" || !strings.HasPrefix(copyCookie.Path, "/") {
			p := u.Path
			i := strings.LastIndex(p, "/")
			if i <= 0 {
				copyCookie.Path = "/"
			} else {
				copyCookie.Path = p[:i]
			}
		}
		if copyCookie.MaxAge > 0 {
			copyCookie.Expires = time.Now().Add(time.Duration(copyCookie.MaxAge) * time.Second)
		}
		domain := strings.TrimPrefix(strings.ToLower(copyCookie.Domain), ".")
		if domain != "" && domain != u.Hostname() {
			continue
		}
		kept := j.Records[:0]
		for _, old := range j.Records {
			if old.Cookie.Name == copyCookie.Name && old.Cookie.Path == copyCookie.Path && strings.TrimPrefix(old.Cookie.Domain, ".") == strings.TrimPrefix(copyCookie.Domain, ".") {
				continue
			}
			kept = append(kept, old)
		}
		j.Records = kept
		if copyCookie.MaxAge < 0 || (!copyCookie.Expires.IsZero() && copyCookie.Expires.Before(time.Now())) {
			continue
		}
		copyCookie.MaxAge = 0
		j.Records = append(j.Records, storedCookie{u.String(), &copyCookie})
	}
}

type client struct {
	HTTP *http.Client
	Jar  *persistentJar
	Path string
}

func cookiePath() (string, error) {
	root := os.Getenv("XDG_STATE_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errors.New("ホームディレクトリを取得できません。")
		}
		root = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(root, "takuma-cli", "cookies.json"), nil
}
func newClient() (*client, error) {
	path, err := cookiePath()
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, errors.New("Cookie保存先を作成できません。")
	}
	if err = os.Chmod(dir, 0700); err != nil {
		return nil, errors.New("Cookie保存先の権限を設定できません。")
	}
	jarBase, _ := cookiejar.New(nil)
	jar := &persistentJar{Jar: jarBase}
	data, err := os.ReadFile(path)
	if err == nil {
		if err = os.Chmod(path, 0600); err != nil {
			return nil, errors.New("Cookieファイルの権限を設定できません。")
		}
		var records []storedCookie
		if json.Unmarshal(data, &records) != nil {
			return nil, errors.New("Cookieファイルが不正です。forgetして再ログインしてください。")
		}
		for _, r := range records {
			u, e := checkedURL(r.Origin)
			if e != nil || r.Cookie == nil {
				return nil, errors.New("Cookieファイルが不正です。")
			}
			jar.SetCookies(u, []*http.Cookie{r.Cookie})
		}
	} else if !os.IsNotExist(err) {
		return nil, errors.New("Cookieファイルを読めません。")
	}
	c := &client{Jar: jar, Path: path}
	c.HTTP = &http.Client{Jar: jar, Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("リダイレクトが多すぎます。")
		}
		if req.Method == http.MethodPost {
			return errors.New("リダイレクトによるPOSTの再送を拒否しました。")
		}
		_, err := checkedURL(req.URL.String())
		return err
	}}
	return c, nil
}
func (c *client) save() error {
	data, err := json.Marshal(c.Jar.Records)
	if err != nil {
		return errors.New("Cookieを保存できません。")
	}
	f, err := os.CreateTemp(filepath.Dir(c.Path), ".cookies-*")
	if err != nil {
		return errors.New("Cookieを保存できません。")
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return errors.New("Cookieを保存できません。")
	}
	if err = f.Close(); err != nil {
		return errors.New("Cookieを保存できません。")
	}
	if os.Rename(f.Name(), c.Path) != nil {
		return errors.New("Cookieを保存できません。")
	}
	return nil
}
func (c *client) request(target string, data []pair) (string, error) {
	u, err := checkedURL(target)
	if err != nil {
		return "", err
	}
	method := "GET"
	var body io.Reader
	if data != nil {
		method = "POST"
		body = strings.NewReader(encodePairs(data))
	}
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return "", errors.New("リクエストを作成できません。")
	}
	req.Header.Set("User-Agent", "takuma-cli/1.0")
	req.Header.Set("Accept", "text/html")
	if data != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", baseURL)
		req.Header.Set("Referer", loginURL)
		if strings.Contains(u.Path, "/regist/") {
			req.Header.Set("Referer", registURL)
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
	b, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024+1))
	if err != nil {
		return "", errors.New("応答を読み取れません。")
	}
	if len(b) > 8*1024*1024 {
		return "", errors.New("応答サイズが大きすぎます。")
	}
	if err = c.save(); err != nil {
		return "", err
	}
	return string(b), nil
}
func (c *client) registration() (*form, error) {
	s, err := c.request(registURL, nil)
	if err != nil {
		return nil, err
	}
	return parseForm(s, "regist")
}
