package cli

import (
	"net/http"
	"net/url"
	"testing"
)

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
