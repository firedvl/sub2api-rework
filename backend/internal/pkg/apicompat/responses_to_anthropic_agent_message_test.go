package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesToAnthropicRejectsV2AgentMessages(t *testing.T) {
	for _, role := range []string{"", "user", "system", "assistant"} {
		t.Run(role, func(t *testing.T) {
			input, err := json.Marshal([]any{map[string]any{
				"type": "agent_message", "role": role, "author": "/root", "recipient": "/root/child",
				"content": []any{map[string]any{"type": "input_text", "text": "Task header"}, map[string]any{"type": "encrypted_content", "encrypted_content": "opaque-task"}},
			}})
			require.NoError(t, err)
			converted, err := ResponsesToAnthropicRequest(&ResponsesRequest{Model: "claude", Input: input})
			require.ErrorContains(t, err, "does not support agent_message")
			require.Nil(t, converted)
		})
	}
}
