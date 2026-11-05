package cli

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDotEnvCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, text, user, pass string
		invalid                bool
	}{
		{name: "plain", text: "TAKUMA_STUDENT_ID=012345\nTAKUMA_PASSWORD=p=a#ss$word\\x", user: "012345", pass: `p=a#ss$word\x`},
		{name: "quoted", text: "# comment\r\nexport TAKUMA_STUDENT_ID = \"012345\"\r\nTAKUMA_PASSWORD=' p# $word\\x ' # comment", user: "012345", pass: ` p# $word\x `},
		{name: "comments", text: "TAKUMA_STUDENT_ID=012345 # id\nTAKUMA_PASSWORD=test # password\nOTHER=anything", user: "012345", pass: "test"},
		{name: "missing", text: "TAKUMA_STUDENT_ID=012345", invalid: true},
		{name: "empty", text: "TAKUMA_STUDENT_ID=012345\nTAKUMA_PASSWORD=''", invalid: true},
		{name: "unclosed", text: "TAKUMA_STUDENT_ID=012345\nTAKUMA_PASSWORD='private-value", invalid: true},
		{name: "trailing", text: "TAKUMA_STUDENT_ID=012345\nTAKUMA_PASSWORD='private-value' garbage", invalid: true},
		{name: "malformed", text: "private-value", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".env")
			if err := os.WriteFile(path, []byte(tc.text), 0600); err != nil {
				t.Fatal(err)
			}
			user, pass, found, err := credentialsFromDotEnv(path)
			if !found || (err != nil) != tc.invalid {
				t.Fatalf("found=%v err=%v", found, err)
			}
			if err != nil {
				if strings.Contains(err.Error(), "private-value") {
					t.Fatal("secret leaked")
				}
				return
			}
			if user != tc.user || pass != tc.pass {
				t.Fatal("credentials differ")
			}
		})
	}
}
func TestLoginCredentialSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	var out bytes.Buffer
	calls := 0
	secret := func() (string, error) { calls++; return "interactive-password", nil }
	user, pass, err := loginCredentials(path, bufio.NewReader(strings.NewReader("interactive-user\n")), &out, secret)
	if err != nil || user != "interactive-user" || pass != "interactive-password" || calls != 1 {
		t.Fatal("interactive fallback failed", err)
	}
	if err = os.WriteFile(path, []byte("TAKUMA_STUDENT_ID=file-user\nTAKUMA_PASSWORD=file-password"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAKUMA_STUDENT_ID", "environment-user")
	t.Setenv("TAKUMA_PASSWORD", "environment-password")
	out.Reset()
	user, pass, err = loginCredentials(path, bufio.NewReader(strings.NewReader("")), &out, secret)
	if err != nil || user != "file-user" || pass != "file-password" || calls != 1 || out.Len() != 0 {
		t.Fatal("file precedence failed", err)
	}
	if err = os.WriteFile(path, []byte("TAKUMA_STUDENT_ID=file-user"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err = loginCredentials(path, bufio.NewReader(strings.NewReader("")), &out, secret)
	if err == nil || calls != 1 {
		t.Fatal("incomplete file should fail without prompting")
	}
}
func TestLoginWithDotEnv(t *testing.T) {
	dir := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Error(err)
		}
	})
	if err = os.WriteFile(".env", []byte("TAKUMA_STUDENT_ID=012345\nTAKUMA_PASSWORD='test +&#password'"), 0600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	fakeNetwork(t, func(r *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			if r.Method != "GET" || r.URL.String() != loginURL {
				t.Error("expected login form")
			}
			return response(r, `<form id=login method=post action="/takuma/student/login.action"><input name=gakusekiNo><input type=password name=loginPass><input type=checkbox name=rememberMe checked><input type=hidden name=token value=TEST><input type=submit name="method:login" value="ログイン"></form>`), nil
		case 2:
			b, _ := io.ReadAll(r.Body)
			v, _ := url.ParseQuery(string(b))
			if r.Method != "POST" || v.Get("gakusekiNo") != "012345" || v.Get("loginPass") != "test +&#password" || v.Get("token") != "TEST" {
				t.Error("incorrect login payload")
			}
			if _, ok := v["rememberMe"]; ok {
				t.Error("rememberMe should be unchecked")
			}
			return response(r, "ok"), nil
		case 3:
			if r.URL.String() != registURL {
				t.Error("expected registration check")
			}
			return response(r, fixture), nil
		default:
			t.Fatal("unexpected request")
			return nil, nil
		}
	})
	var out bytes.Buffer
	if err = Run([]string{"login"}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if requests != 3 || strings.Contains(out.String(), "学籍番号:") || strings.Contains(out.String(), "password") {
		t.Fatal("login should complete without prompting or printing credentials")
	}
}
