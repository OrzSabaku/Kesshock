package cli

import (
	"testing"
)

func TestURLValidation(t *testing.T) {
	for _, target := range []string{"https://example.com/", "http://gaihaku.mogsmile.com/takuma/student/", "https://gaihaku.mogsmile.com/elsewhere", "https://user@gaihaku.mogsmile.com/takuma/student/", "/takuma/student/../../other"} {
		if _, err := validateURL(target); err == nil {
			t.Fatal(target)
		}
	}
	u, err := resolveURL(registrationURL, "regist.action")
	if err != nil || u.String() != registrationURL {
		t.Fatal(u, err)
	}
}
