package cli

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
		// Persist an absolute expiry; replaying MaxAge would extend cookie life.
		copyCookie.MaxAge = 0
		j.Records = append(j.Records, storedCookie{u.String(), &copyCookie})
	}
}

// cookiePath keeps the original directory so renaming the executable does not
// invalidate existing sessions or leave old cookies behind after forget.
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

// loadCookieJar restricts filesystem access before restoring session cookies.
func loadCookieJar(path string) (*persistentJar, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, errors.New("Cookie保存先を作成できません。")
	}
	if err := os.Chmod(dir, 0700); err != nil {
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
			u, e := validateURL(r.Origin)
			if e != nil || r.Cookie == nil {
				return nil, errors.New("Cookieファイルが不正です。")
			}
			jar.SetCookies(u, []*http.Cookie{r.Cookie})
		}
	} else if !os.IsNotExist(err) {
		return nil, errors.New("Cookieファイルを読めません。")
	}
	return jar, nil
}

// save replaces the cookie file atomically. CreateTemp creates it with mode 0600,
// and a failed write leaves the previous session file intact.
func (c *siteClient) save() error {
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
