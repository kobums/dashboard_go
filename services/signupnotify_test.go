package services

import (
	"testing"

	"dashboard/controllers/rest"
)

func TestSignupNotifyMessage(t *testing.T) {
	title, body, click := signupNotifyMessage([]rest.NewSignup{{Key: "fotstat", Name: "Fotstat", Count: 2, Today: 3}})
	if title != "신규 가입 +2" || body != "Fotstat +2 (오늘 3명)" || click != dashboardURL+"/services/fotstat" {
		t.Fatalf("single: %q %q %q", title, body, click)
	}

	title, body, click = signupNotifyMessage([]rest.NewSignup{
		{Key: "fotstat", Name: "Fotstat", Count: 2, Today: 3},
		{Key: "snippet", Name: "Snippet", Count: 1, Today: 1},
	})
	if title != "신규 가입 +3" || body != "Fotstat +2 (오늘 3명)\nSnippet +1 (오늘 1명)" || click != dashboardURL+"/services" {
		t.Fatalf("multi: %q %q %q", title, body, click)
	}
}
