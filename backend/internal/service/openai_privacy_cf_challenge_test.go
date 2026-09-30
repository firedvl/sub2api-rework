package service

import "testing"

func TestIsCloudflareChallengeResponse(test *testing.T) {
	for _, scenario := range []struct {
		name        string
		cfMitigated string
		body        string
		want        bool
	}{
		{"challenge header without body markers", "challenge", `<html>...</html>`, true},
		{"case-insensitive trimmed header", " Challenge ", "", true},
		{"body marker only", "", "<title>Just a moment...</title>", true},
		{"cloudflare body marker", "", "cloudflare blocked", true},
		{"cf body marker", "", "cf-ray", true},
		{"plain JSON failure", "", `{"detail":"Unauthorized"}`, false},
		{"non-challenge header", "other", "", false},
		{"empty", "", "", false},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			if got := isCloudflareChallengeResponse(scenario.cfMitigated, scenario.body); got != scenario.want {
				test.Fatalf("isCloudflareChallengeResponse(%q, %q) = %v, want %v", scenario.cfMitigated, scenario.body, got, scenario.want)
			}
		})
	}
}
