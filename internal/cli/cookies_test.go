package cli

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCookiePersistence(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	c, err := newClient()
	if err != nil {
		t.Fatal(err)
	}
	u, _ := validateURL(loginURL)
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
