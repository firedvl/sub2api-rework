package service

import (
	"encoding/json"
	"errors"
)

// ResearchOAuthTransform uses the deployed normal OAuth branch, not passthrough.
// It does not attest provider output limits or grant live dispatch authority.
func ResearchOAuthTransform(body []byte) ([]byte, error) {
	var request map[string]any
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, err
	}
	if request["model"] != "gpt-6.1-sol" {
		return nil, errors.New("research model mismatch")
	}
	result := applyCodexOAuthTransformWithOptions(request, codexOAuthTransformOptions{
		OmitPromotedSystemMessagesFromInput: true,
	})
	if result.Error != nil {
		return nil, result.Error
	}
	if request["model"] != "gpt-6.1-sol" || request["stream"] != true || request["store"] != false {
		return nil, errors.New("research transform binding changed")
	}
	return json.Marshal(request)
}
