package apicompat

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSonnet55ResponsesThinkingAndSampling(t *testing.T) {
	for _, effort := range []string{"", "low", "medium", "high", "xhigh", "max", "none"} {
		req := &ResponsesRequest{Model: "claude-sonnet-5-5", Input: json.RawMessage(`"hello"`), Reasoning: &ResponsesReasoning{Effort: effort}}
		out, err := ResponsesToAnthropicRequest(req)
		require.NoError(t, err, effort)
		require.Zero(t, out.Thinking.BudgetTokens)
		if effort == "none" {
			require.Equal(t, "between_tools", out.Thinking.Type)
			require.Equal(t, "low", out.OutputConfig.Effort)
		} else {
			require.Equal(t, "adaptive", out.Thinking.Type)
			if effort == "" {
				effort = "high"
			}
			require.Equal(t, effort, out.OutputConfig.Effort)
		}
	}
	for _, choice := range []string{`"required"`, `{"type":"function","name":"lookup"}`} {
		_, err := ResponsesToAnthropicRequest(&ResponsesRequest{Model: "claude-sonnet-5-5", Input: json.RawMessage(`"hello"`), ToolChoice: json.RawMessage(choice)})
		require.ErrorContains(t, err, "forced tool_choice")
	}
	temperature := 0.7
	_, err := ResponsesToAnthropicRequest(&ResponsesRequest{Model: "claude-sonnet-5-5", Input: json.RawMessage(`"hello"`), Temperature: &temperature})
	require.ErrorContains(t, err, "temperature")
	topP := 0.5
	_, err = ResponsesToAnthropicRequest(&ResponsesRequest{Model: "claude-sonnet-5-5", Input: json.RawMessage(`"hello"`), TopP: &topP})
	require.ErrorContains(t, err, "top_p")
	_, err = ResponsesToAnthropicRequest(&ResponsesRequest{Model: "claude-sonnet-5-5", Input: json.RawMessage(`"hello"`), Reasoning: &ResponsesReasoning{Effort: "minimal"}})
	require.ErrorContains(t, err, "reasoning effort")

	temperature, topP = 1, 0.99
	_, err = ResponsesToAnthropicRequest(&ResponsesRequest{Model: "claude-sonnet-5-5", Input: json.RawMessage(`"hello"`), Temperature: &temperature, TopP: &topP})
	require.NoError(t, err)
}

func TestSonnet55SignedThinkingResponsesRoundTrip(t *testing.T) {
	block := AnthropicContentBlock{Type: "thinking", Thinking: "", Signature: "signed-sonnet-block"}
	response := AnthropicToResponsesResponse(&AnthropicResponse{Model: "claude-sonnet-5-5", Content: []AnthropicContentBlock{block, {Type: "text", Text: "progress"}, {Type: "tool_use", ID: "toolu_1", Name: "lookup", Input: json.RawMessage(`{}`)}}})
	require.Len(t, response.Output, 3)
	require.NotEmpty(t, response.Output[0].EncryptedContent)
	require.Equal(t, "message", response.Output[1].Type)
	raw, err := json.Marshal(response.Output)
	require.NoError(t, err)
	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(raw, &items))
	items = append(items, ResponsesInputItem{Type: "function_call_output", CallID: response.Output[2].CallID, Output: "ok"})
	raw, err = json.Marshal(items)
	require.NoError(t, err)
	converted, err := ResponsesToAnthropicRequest(&ResponsesRequest{Model: "claude-sonnet-5-5", Input: raw})
	require.NoError(t, err)
	require.Len(t, converted.Messages, 2)
	var blocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(converted.Messages[0].Content, &blocks))
	require.Equal(t, block, blocks[0])
	require.Equal(t, "text", blocks[1].Type)
	require.Equal(t, "tool_use", blocks[2].Type)
}
