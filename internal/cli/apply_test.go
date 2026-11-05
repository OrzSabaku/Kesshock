package cli

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

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
				if v.Get("token") != "FRESH" || v.Get(applicationFields[0]) != "2026/11/14" || v.Get(applicationFields[3]) != "2026/11/13" {
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
			err := Run(args, strings.NewReader(answer), &out)
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
