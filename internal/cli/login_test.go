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
