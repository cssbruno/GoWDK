package i18nexample

import "testing"

func TestHomeMessagesComplete(t *testing.T) {
	if report := homeMessages.Check(homeRequiredMessages); !report.OK() {
		t.Fatalf("localized message catalogs are incomplete:\n%s", report.Error())
	}
}
