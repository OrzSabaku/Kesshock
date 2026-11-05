package cli

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

const otpFixture = `<form id="tlogin" action="/takuma/student/tlogin.action" method="post"><input type="hidden" name="struts.token.name" value="token"><input type="hidden" name="token" value="OTP-TOKEN"><input type="text" name="authCode" maxlength="10"><input type="submit" name="method:confirm" value="確認"></form>`

func TestOTPConfirmation(t *testing.T) {
	for _, scenario := range []string{"success", "rejected", "expired-session", "empty", "eof", "missing-field", "unsafe-action", "no-otp", "password-rejected"} {
		t.Run(scenario, func(t *testing.T) {
			posts := 0
			fakeNetwork(t, func(r *http.Request) (*http.Response, error) {
				posts++
				if r.Method != "POST" || r.URL.String() != baseURL+"/takuma/student/tlogin.action" {
					t.Fatal("unexpected request", r.Method, r.URL)
				}
				b, _ := io.ReadAll(r.Body)
				v, _ := url.ParseQuery(string(b))
				if v.Get("authCode") != "123456" || v.Get("token") != "OTP-TOKEN" || v.Get("struts.token.name") != "token" || v.Get("method:confirm") != "確認" {
					t.Fatal("incorrect OTP submission")
				}
				body := fixture
				if scenario == "rejected" {
					body = otpFixture
				}
				if scenario == "expired-session" {
					body = `<form id="login" method="post"></form>`
				}
				return response(r, body), nil
			})
			c, err := newClient()
			if err != nil {
				t.Fatal(err)
			}
			source, answer := otpFixture, " 123456 \n"
			switch scenario {
			case "empty":
				answer = "\n"
			case "eof":
				answer = ""
			case "missing-field":
				source = strings.ReplaceAll(source, "authCode", "unknown")
			case "unsafe-action":
				source = strings.ReplaceAll(source, "/takuma/student/tlogin.action", "https://example.com/otp")
			case "no-otp":
				source = fixture
			case "password-rejected":
				source = `<form id="login" method="post"></form>`
			}
			var out bytes.Buffer
			err = c.confirmOTP(source, bufio.NewReader(strings.NewReader(answer)), &out)
			wantError := scenario != "success" && scenario != "no-otp"
			if (err != nil) != wantError {
				t.Fatalf("error=%v", err)
			}
			wantPosts := 0
			if scenario == "success" || scenario == "rejected" || scenario == "expired-session" {
				wantPosts = 1
			}
			if posts != wantPosts {
				t.Fatalf("posts=%d want %d", posts, wantPosts)
			}
			if strings.Contains(out.String(), "123456") {
				t.Fatal("code printed")
			}
			if data, err := os.ReadFile(c.Path); err == nil && strings.Contains(string(data), "123456") {
				t.Fatal("code persisted")
			}
		})
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
