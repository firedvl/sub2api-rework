package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesToChatCompletionsRequest_AgentMessagesPreserveTaskAndReply(test *testing.T) {
	request := &ResponsesRequest{Model: "custom-chat", Input: json.RawMessage(`[
		{"type":"message","role":"user","content":"start"},
		{"type":"agent_message","content":[
			{"type":"input_text","text":"Task:\n"},
			{"type":"encrypted_content","encrypted_content":"Reply ALPHA"}
		]},
		{"type":"reasoning","summary":[{"type":"summary_text","text":"just answer"}]},
		{"type":"message","role":"assistant","content":"ALPHA"},
		{"type":"agent_message","content":[{"type":"text","text":"Result: ALPHA"}]}
	]`)}
	converted, err := ResponsesToChatCompletionsRequest(request)
	require.NoError(test, err)
	require.Len(test, converted.Messages, 4)
	require.Equal(test, []string{"user", "user", "assistant", "user"}, chatMessageRoles(converted.Messages))
	require.JSONEq(test, `"Task:\nReply ALPHA"`, string(converted.Messages[1].Content))
	require.JSONEq(test, `"ALPHA"`, string(converted.Messages[2].Content))
	require.JSONEq(test, `"Result: ALPHA"`, string(converted.Messages[3].Content))
}

func TestResponsesToChatCompletionsRequest_AgentMessagesSkipEmptyContent(test *testing.T) {
	request := &ResponsesRequest{Model: "custom-chat", Input: json.RawMessage(`[
		{"type":"message","role":"user","content":"hi"},
		{"type":"agent_message","content":[]},
		{"type":"agent_message"},
		{"type":"agent_message","content":[{"type":"input_image","image_url":"data:image/png;base64,AA=="}]}
	]`)}
	converted, err := ResponsesToChatCompletionsRequest(request)
	require.NoError(test, err)
	require.Len(test, converted.Messages, 1)
	require.Equal(test, "user", converted.Messages[0].Role)
}

func TestAgentMessageTextContentForms(test *testing.T) {
	for _, scenario := range []struct {
		content string
		want    string
	}{
		{`"  task  "`, "  task  "},
		{`null`, ""},
		{`{"text":"not a content part array"}`, ""},
		{`[{"type":"input_text","text":"header"},{"type":"encrypted_content","encrypted_content":"body"},{"type":"text","text":"tail"}]`, "headerbodytail"},
	} {
		require.Equal(test, scenario.want, agentMessageText(json.RawMessage(scenario.content)))
	}
}

func TestResponsesToChatCompletionsRequest_AdditionalToolsRemainChatCompatible(test *testing.T) {
	var request ResponsesRequest
	require.NoError(test, json.Unmarshal([]byte(`{
		"model":"custom-chat","parallel_tool_calls":false,"reasoning":{"effort":"medium"},
		"input":[
			{"type":"additional_tools","tools":[
				{"type":"custom","name":"exec","format":{"type":"grammar","syntax":"lark","definition":"start: /.*/"}},
				{"type":"function","name":"wait","parameters":{"type":"object"}},
				{"type":"namespace","name":"collaboration","tools":[{"type":"function","name":"spawn_agent","parameters":{"type":"object"}}]}
			]},
			{"type":"message","role":"user","content":"reply ok"}
		]
	}`), &request))
	converted, err := ResponsesToChatCompletionsRequest(&request)
	require.NoError(test, err)
	require.Len(test, converted.Messages, 1)
	require.Len(test, converted.Tools, 3)
	names := make([]string, 0, len(converted.Tools))
	for _, tool := range converted.Tools {
		require.Equal(test, "function", tool.Type)
		require.NotNil(test, tool.Function)
		names = append(names, tool.Function.Name)
	}
	require.Equal(test, []string{"exec", "wait", "collaboration__spawn_agent"}, names)
	require.JSONEq(test, customToolInputSchema, string(converted.Tools[0].Function.Parameters))
	require.NotNil(test, converted.ParallelToolCalls)
	require.False(test, *converted.ParallelToolCalls)
	require.Equal(test, "medium", converted.ReasoningEffort)
}
