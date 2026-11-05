package cli

import (
	"net/url"
	"strings"
	"testing"
)

func TestChangePreservesPayload(t *testing.T) {
	f := mustForm(t, fixture)
	before, after, err := applyFormChange(f, "2026/11/14", applicationFields[:1], true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sameSchedule(before, after) {
		t.Fatal("change missing")
	}
	encoded := encodePairs(f.payload())
	values, err := url.ParseQuery(encoded)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"remarkList": "帰省 & 帰宅", "kesshokuDinnerDate": "2026/11/13", "gaihakuDate": "2026/11/13", "kesshokuMorningDate": "2026/11/14", "method:regist": "登録"} {
		if values.Get(key) != want {
			t.Fatalf("%s: %q", key, values.Get(key))
		}
	}
	if strings.Join(values["dateStrList"], ",") != "2026/11/13,2026/11/14" {
		t.Fatal(values)
	}
	if strings.Index(encoded, "dateStrList=2026%2F11%2F13") > strings.Index(encoded, "remarkList=") {
		t.Fatal("input order changed")
	}
	if _, ok := values[applicationFields[1]]; ok {
		t.Fatal("disabled input sent")
	}
}

func TestChangeRejectsUnavailable(t *testing.T) {
	for _, tc := range []struct{ date, field string }{{"2026/11/13", applicationFields[0]}, {"2026/11/14", applicationFields[1]}, {"2026/11/15", applicationFields[3]}} {
		if _, _, err := applyFormChange(mustForm(t, fixture), tc.date, []string{tc.field}, true, nil); err == nil {
			t.Fatal(tc)
		}
	}
}

func TestRemoveRemarkAndNoop(t *testing.T) {
	f := mustForm(t, fixture)
	a, b, err := applyFormChange(f, "2026/11/13", applicationFields[3:], true, nil)
	if err != nil || !sameSchedule(a, b) {
		t.Fatal("expected noop", err)
	}
	remark := "変更 + & 😀"
	_, after, err := applyFormChange(f, "2026/11/13", applicationFields[3:], false, &remark)
	if err != nil {
		t.Fatal(err)
	}
	if after[0].Fields[applicationFields[3]].Checked || after[0].Remark != remark {
		t.Fatal(after)
	}
	if strings.Contains(encodePairs(f.payload()), "gaihakuDate=") {
		t.Fatal("removed checkbox sent")
	}
	short := mustForm(t, strings.Replace(fixture, `maxlength="100"`, `maxlength="1"`, 1))
	emoji := "😀"
	if _, _, err = applyFormChange(short, "2026/11/13", applicationFields[3:], false, &emoji); err == nil {
		t.Fatal("UTF-16 length not enforced")
	}
}

func TestInvalidRemarkLeavesFormUnchanged(t *testing.T) {
	for _, attribute := range []string{`type="text" maxlength="1"`, `type="text" maxlength="invalid"`, `type="text" disabled`, `type="hidden"`} {
		t.Run(attribute, func(t *testing.T) {
			source := strings.Replace(fixture, `type="text" name="remarkList" value="帰省 &amp; 帰宅" maxlength="100"`, `name="remarkList" value="帰省 &amp; 帰宅" `+attribute, 1)
			form := mustForm(t, source)
			before := encodePairs(form.payload())
			remark := "長すぎる備考"
			if _, _, err := applyFormChange(form, "2026/11/13", applicationFields[3:], false, &remark); err == nil {
				t.Fatal("expected validation error")
			}
			if actual := encodePairs(form.payload()); actual != before {
				t.Fatal("rejected edit changed the form")
			}
		})
	}
}
