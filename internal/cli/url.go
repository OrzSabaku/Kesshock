package cli

import (
	"errors"
	"net/url"
	"strings"
)

const baseURL = "https://gaihaku.mogsmile.com"
const loginURL = baseURL + "/takuma/student/login!init.action"
const registrationURL = baseURL + "/takuma/student/regist/regist.action"

func validateURL(target string) (*url.URL, error) { return resolveURL(baseURL, target) }

// resolveURL applies the same origin and path policy to form actions and redirects.
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
