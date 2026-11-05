package cli

import (
	"io"
	"net/http"
	"strings"
	"testing"
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

type transportFunc func(*http.Request) (*http.Response, error)

func mustForm(t *testing.T, s string) *htmlForm {
	t.Helper()
	f, err := parseForm(s, "regist")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// fakeNetwork replaces process-wide HTTP state, so callers must not use t.Parallel.
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
