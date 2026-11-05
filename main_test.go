package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fixture = `<form id="regist" action="/takuma/student/regist/regist.action" method="post">
<input type="hidden" name="struts.token.name" value="token"><input type="hidden" name="token" value="OLD">
<input type="hidden" name="dateStrList" value="2026/11/13" id="duplicate">
<input type="checkbox" name="kesshokuDinnerDate" value="2026/11/13" checked>
<input type="checkbox" name="gaihakuDate" value="2026/11/13" checked>
<input type="text" name="remarkList" value="帰省 &amp; 帰宅" maxlength="100">
<input type="hidden" name="dateStrList" value="2026/11/14" id="duplicate">
<input type="checkbox" name="kesshokuMorningDate" value="2026/11/14">
<input type="checkbox" name="kesshokuLunchDate" value="2026/11/14" disabled>
<input type="text" name="remarkList" value="">
<input type="submit" name="method:regist" value="登録"></form>`

func mustForm(t *testing.T, s string) *form {
	t.Helper()
	f, err := parseForm(s, "regist")
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func TestChangePreservesPayload(t *testing.T) {
	f := mustForm(t, fixture)
	before, after, err := change(f, "2026/11/14", fieldNames[:1], true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sameState(before, after) {
		t.Fatal("change missing")
	}
	encoded := encodePairs(f.payload())
	values, err := url.ParseQuery(encoded)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"remarkList": "帰省 & 帰宅", "kesshokuDinnerDate": "2026/11/13", "gaihakuDate": "2026/11/13", "kesshokuMorningDate": "2026/11/14", "method:regist": "登録"} {
		if values.Get(key) != want {
			t.Fatalf("%s: %q", key, values.Get(key))
		}
	}
	if strings.Join(values["dateStrList"], ",") != "2026/11/13,2026/11/14" {
		t.Fatal(values)
	}
	if strings.Index(encoded, "dateStrList=2026%2F11%2F13") > strings.Index(encoded, "remarkList=") {
		t.Fatal("input order changed")
	}
	if _, ok := values[fieldNames[1]]; ok {
		t.Fatal("disabled input sent")
	}
}
func TestChangeRejectsUnavailable(t *testing.T) {
	for _, tc := range []struct{ date, field string }{{"2026/11/13", fieldNames[0]}, {"2026/11/14", fieldNames[1]}, {"2026/11/15", fieldNames[3]}} {
		if _, _, err := change(mustForm(t, fixture), tc.date, []string{tc.field}, true, nil); err == nil {
			t.Fatal(tc)
		}
	}
}
func TestRemoveRemarkAndNoop(t *testing.T) {
	f := mustForm(t, fixture)
	a, b, err := change(f, "2026/11/13", fieldNames[3:], true, nil)
	if err != nil || !sameState(a, b) {
		t.Fatal("expected noop", err)
	}
	remark := "変更 + & 😀"
	_, after, err := change(f, "2026/11/13", fieldNames[3:], false, &remark)
	if err != nil {
		t.Fatal(err)
	}
	if after[0].Fields[fieldNames[3]].Checked || after[0].Remark != remark {
		t.Fatal(after)
	}
	if strings.Contains(encodePairs(f.payload()), "gaihakuDate=") {
		t.Fatal("removed checkbox sent")
	}
	short := mustForm(t, strings.Replace(fixture, `maxlength="100"`, `maxlength="1"`, 1))
	emoji := "😀"
	if _, _, err = change(short, "2026/11/13", fieldNames[3:], false, &emoji); err == nil {
		t.Fatal("UTF-16 length not enforced")
	}
}
func TestHTMLParsing(t *testing.T) {
	source := `<!-- <form id="login" method="post"> --><script>var x='<form id="login" method="post">';</script><FORM id=login method=POST action='/takuma/student/login.action'><input name=gakusekiNo><input type=password name=loginPass><input type=checkbox name=rememberMe value=true><input type=hidden name=__checkbox_rememberMe value=true><input name=note value="a > b &amp; c"></FORM>`
	f, err := parseForm(source, "login")
	if err != nil {
		t.Fatal(err)
	}
	v, _ := url.ParseQuery(encodePairs(f.payload()))
	if v.Get("note") != "a > b & c" || v.Get("__checkbox_rememberMe") != "true" {
		t.Fatal(v)
	}
	if _, ok := v["rememberMe"]; ok {
		t.Fatal(v)
	}
	for _, s := range []string{fixture, `<form id=login method=get></form>`, `<form id=login method=post></form><form id=login method=post></form>`} {
		if _, err := parseForm(s, "login"); err == nil {
			t.Fatal("invalid form accepted")
		}
	}
}
func TestURLValidation(t *testing.T) {
	for _, target := range []string{"https://example.com/", "http://gaihaku.mogsmile.com/takuma/student/", "https://gaihaku.mogsmile.com/elsewhere", "https://user@gaihaku.mogsmile.com/takuma/student/", "/takuma/student/../../other"} {
		if _, err := checkedURL(target); err == nil {
			t.Fatal(target)
		}
	}
	u, err := resolveURL(registURL, "regist.action")
	if err != nil || u.String() != registURL {
		t.Fatal(u, err)
	}
}
func TestCookiePersistence(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	c, err := newClient()
	if err != nil {
		t.Fatal(err)
	}
	u, _ := checkedURL(loginURL)
	c.Jar.SetCookies(u, []*http.Cookie{{Name: "session", Value: "test-only", Path: "/takuma/student/", Secure: true, HttpOnly: true}, {Name: "expired", Value: "gone", Expires: time.Now().Add(-time.Hour)}})
	if err = c.save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := newClient()
	if err != nil {
		t.Fatal(err)
	}
	cookies := loaded.Jar.Cookies(u)
	if len(cookies) != 1 || cookies[0].Value != "test-only" {
		t.Fatal(cookies)
	}
	for p, mode := range map[string]os.FileMode{c.Path: 0600, filepath.Dir(c.Path): 0700} {
		info, err := os.Stat(p)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatal(p, err)
		}
	}
	c.Jar.SetCookies(u, []*http.Cookie{{Name: "session", Path: "/takuma/student/", MaxAge: -1}})
	if err = c.save(); err != nil {
		t.Fatal(err)
	}
	loaded, err = newClient()
	if err != nil || len(loaded.Jar.Cookies(u)) != 0 {
		t.Fatal("deleted cookie restored", err)
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fakeNetwork(t *testing.T, fn transportFunc) {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = fn
	t.Cleanup(func() { http.DefaultTransport = old })
	t.Setenv("XDG_STATE_HOME", t.TempDir())
}
func response(r *http.Request, body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}
}
func TestApplyWorkflow(t *testing.T) {
	for _, scenario := range []string{"success", "conflict", "post-failure", "mismatch", "cancel", "dry-run", "yes", "short-yes", "yes-dry-run", "yes-conflict"} {
		t.Run(scenario, func(t *testing.T) {
			gets, posts := 0, 0
			fakeNetwork(t, func(r *http.Request) (*http.Response, error) {
				if r.Method == "GET" {
					gets++
					body := fixture
					if gets >= 2 {
						body = strings.ReplaceAll(body, "OLD", "FRESH")
					}
					if (scenario == "conflict" || scenario == "yes-conflict") && gets == 2 {
						body = strings.Replace(body, "帰省", "競合", 1)
					}
					if gets == 3 && scenario != "mismatch" {
						body = strings.Replace(body, `name="kesshokuMorningDate"`, `checked name="kesshokuMorningDate"`, 1)
					}
					return response(r, body), nil
				}
				posts++
				b, _ := io.ReadAll(r.Body)
				v, _ := url.ParseQuery(string(b))
				if v.Get("token") != "FRESH" || v.Get(fieldNames[0]) != "2026/11/14" || v.Get(fieldNames[3]) != "2026/11/13" {
					t.Errorf("unexpected submission: %v", v)
				}
				if scenario == "post-failure" {
					return nil, errors.New("offline")
				}
				return response(r, "ok"), nil
			})
			args := []string{"kesshoku", "add", "2026-11-14", "--meal", "morning"}
			if scenario != "dry-run" && scenario != "yes-dry-run" {
				args = append(args, "--apply")
			}
			answer := "y\n"
			if strings.HasPrefix(scenario, "yes") || scenario == "short-yes" {
				flag := "--yes"
				if scenario == "short-yes" {
					flag = "-y"
				}
				args = append(args, flag)
				answer = ""
			}
			if scenario == "cancel" {
				answer = "n\n"
			}
			var out bytes.Buffer
			err := run(args, strings.NewReader(answer), &out)
			expectError := scenario == "conflict" || scenario == "yes-conflict" || scenario == "post-failure" || scenario == "mismatch"
			if (err != nil) != expectError {
				t.Fatalf("err=%v output=%s", err, &out)
			}
			wantPosts := 1
			if scenario == "conflict" || scenario == "yes-conflict" || scenario == "cancel" || scenario == "dry-run" || scenario == "yes-dry-run" {
				wantPosts = 0
			}
			if posts != wantPosts {
				t.Fatalf("posts=%d want=%d", posts, wantPosts)
			}
		})
	}
}
func TestOfflineStatusAndArgs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "form.html")
	if err := os.WriteFile(path, []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run([]string{"status", "--html", path}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "2026/11/13") {
		t.Fatal(&out)
	}
	for _, args := range [][]string{{"kesshoku", "add", "2026-11-14"}, {"gaihaku", "add", "2026-02-30"}, {"login", "extra"}, {"status", "--unknown"}} {
		if _, err := parseArgs(args); err == nil {
			t.Fatal(args)
		}
	}
}
func TestRedirectRejection(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	c, err := newClient()
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse("https://example.com/")
	if err = c.HTTP.CheckRedirect(&http.Request{URL: u}, nil); err == nil {
		t.Fatal("cross-origin redirect accepted")
	}
}
