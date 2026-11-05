package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

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
			if r.URL.String() != registrationURL {
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

// Exercise the full login sequence so an OTP response cannot be mistaken for
// an authenticated session just because the password POST returned HTTP 200.
func TestLoginWithEmailOTP(t *testing.T) {
	for _, rejected := range []bool{false, true} {
		name := "success"
		if rejected {
			name = "rejected"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			previous, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chdir(previous); err != nil {
					t.Error(err)
				}
			})
			if err := os.WriteFile(".env", []byte("TAKUMA_STUDENT_ID=test-user\nTAKUMA_PASSWORD=test-password"), 0600); err != nil {
				t.Fatal(err)
			}
			requests := 0
			fakeNetwork(t, func(r *http.Request) (*http.Response, error) {
				requests++
				switch requests {
				case 1:
					return response(r, `<form id="login" method="post" action="/takuma/student/login.action"><input name="gakusekiNo"><input name="loginPass" type="password"></form>`), nil
				case 2:
					if r.Method != http.MethodPost {
						t.Fatal("expected password submission")
					}
					return response(r, otpFixture), nil
				case 3:
					if r.Method != http.MethodPost || r.URL.Path != "/takuma/student/tlogin.action" {
						t.Fatal("expected OTP submission")
					}
					if rejected {
						return response(r, otpFixture), nil
					}
					return response(r, "ok"), nil
				case 4:
					if rejected || r.Method != http.MethodGet || r.URL.String() != registrationURL {
						t.Fatal("unexpected registration check")
					}
					return response(r, fixture), nil
				default:
					t.Fatal("unexpected retry")
					return nil, nil
				}
			})
			var out bytes.Buffer
			err = Run([]string{"login"}, strings.NewReader("123456\n"), &out)
			if (err != nil) != rejected {
				t.Fatalf("error=%v", err)
			}
			if strings.Contains(out.String(), "ログインを確認しました。") == rejected {
				t.Fatal("incorrect success message")
			}
			wantRequests := 4
			if rejected {
				wantRequests = 3
			}
			if requests != wantRequests {
				t.Fatalf("requests=%d want=%d", requests, wantRequests)
			}
		})
	}
}
