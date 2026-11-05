package cli

import (
	"net/url"
	"testing"
)

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
