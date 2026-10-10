package securityaudit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func inlineFileRequest(t *testing.T, protocol, text string) Request {
	t.Helper()
	fileData := "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte(text))
	part := map[string]any{"type": "input_file", "file_data": fileData}
	field := "input"
	if protocol == "openai_chat_completions" {
		field = "messages"
		part = map[string]any{"type": "file", "file": map[string]any{"file_data": fileData}}
	}
	body, err := json.Marshal(map[string]any{field: []any{map[string]any{"role": "user", "content": []any{part}}}})
	require.NoError(t, err)
	return Request{Protocol: protocol, Body: body}
}

func TestPromptSnapshotInlineTextFiles(t *testing.T) {
	const text = "POLICY_BLOCK \u4e2d\u6587 email@example.com sk-secretvalue123"
	for _, protocol := range []string{"openai_chat_completions", "openai_responses"} {
		t.Run(protocol, func(t *testing.T) {
			req := inlineFileRequest(t, protocol, text)
			snapshot, err := ExtractPromptSnapshot(req)
			require.NoError(t, err)
			require.Equal(t, text, snapshot.ScanText)
			require.Equal(t, text, snapshot.FullPrompt)
			require.Equal(t, 1, snapshot.MessageCount)
			require.NotContains(t, snapshot.RedactedPreview, "email@example.com")
			require.NotContains(t, snapshot.RedactedPreview, "secretvalue123")
			require.NotContains(t, snapshot.ScanText, "data:text/plain")
			blocking, err := ExtractBlockingPromptSnapshot(req, true)
			require.NoError(t, err)
			require.Equal(t, snapshot.PromptHash, blocking.PromptHash)
			require.Equal(t, text, blocking.ScanText)
		})
	}
}

func TestPromptSnapshotInlineFilesKeepTranscriptAndLatestTurnBoundaries(t *testing.T) {
	fileData := "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte("latest file"))
	req := Request{Protocol: "openai_responses", Body: []byte(`{"input":[{"role":"user","content":"old user"},{"role":"assistant","content":"previous output"},{"role":"user","content":[{"type":"input_file","file_data":"` + fileData + `"}]}]}`)}
	full, err := ExtractPromptSnapshot(req)
	require.NoError(t, err)
	require.Contains(t, full.ScanText, "old user")
	require.Contains(t, full.ScanText, "previous output")
	require.True(t, strings.HasPrefix(full.ScanText, "latest file"))
	blocking, err := ExtractBlockingPromptSnapshot(req, true)
	require.NoError(t, err)
	require.Contains(t, blocking.ScanText, "latest file")
	require.Contains(t, blocking.ScanText, "previous output")
	require.NotContains(t, blocking.ScanText, "old user")

	req.Protocol = "responses_websocket"
	req.Body = []byte(`{"type":"response.create","response":` + string(req.Body) + `}`)
	websocket, err := ExtractPromptSnapshot(req)
	require.NoError(t, err)
	require.Equal(t, full.PromptHash, websocket.PromptHash)
}

func TestPromptSnapshotOpaqueInlineFiles(t *testing.T) {
	for _, data := range []string{"data:application/pdf;base64,cGxhaW4=", "data:text/plain;base64,/w==", "data:text/plain;base64,???", "data:text/plain;base64,IAo=", "https://example.test/file.txt"} {
		for _, content := range []string{
			`{"type":"input_file","file_data":"` + data + `"}`,
			`{"type":"file","file":{"file_data":"` + data + `"}}`,
		} {
			_, err := ExtractPromptSnapshot(Request{Protocol: "openai_chat_completions", Body: []byte(`{"messages":[{"role":"user","content":[` + content + `]}]}`)})
			require.ErrorIs(t, err, ErrNoPromptText)
		}
	}
}

func TestPromptSnapshotInlineFileTypedFields(t *testing.T) {
	for _, part := range []string{
		`{"TYPE":"input_file","FILE_DATA":"data:text/plain;base64,YmxvY2tlZCBmaWxl"}`,
		`{"type":"input_file","file_data":"data:text/plain;base64,b3JkaW5hcnkgZmlsZQ==","file_data":"data:text/plain;base64,YmxvY2tlZCBmaWxl"}`,
		`{"type":"input_file","type":null,"file_data":"data:text/plain;base64,YmxvY2tlZCBmaWxl","file_data":null}`,
		`{"TYPE":"file","FILE":{"FILE_DATA":"data:text/plain;base64,YmxvY2tlZCBmaWxl"}}`,
		`{"type":"file","file":{"file_data":"data:text/plain;base64,YmxvY2tlZCBmaWxl","file_data":null}}`,
		`{"type":"file","file":{"file_data":"data:text/plain;base64,YmxvY2tlZCBmaWxl"},"file":{"filename":"new name"}}`,
	} {
		for _, protocol := range []string{"openai_chat_completions", "openai_responses"} {
			field := "input"
			if protocol == "openai_chat_completions" {
				field = "messages"
			}
			req := Request{Protocol: protocol, Body: []byte(`{"` + field + `":[{"role":"user","content":[` + part + `]}]}`)}
			for _, latestOnly := range []bool{false, true} {
				snapshot, err := ExtractBlockingPromptSnapshot(req, latestOnly)
				require.NoError(t, err)
				require.Equal(t, "blocked file", snapshot.ScanText)
			}
		}
	}
}

func TestPromptSnapshotInlineFileContainers(t *testing.T) {
	const blocked = `[{"type":"input_file","file_data":"data:text/plain;base64,YmxvY2tlZCBmaWxl"}]`
	const ordinary = `[{"type":"input_file","file_data":"data:text/plain;base64,b3JkaW5hcnk="}]`
	for _, message := range []string{
		`{"role":"user","Content":` + blocked + `}`,
		`{"role":"user","content":` + ordinary + `,"content":` + blocked + `}`,
		`{"ROLE":"user","content":` + blocked + `}`,
		`{"role":"other","content":` + blocked + `}`,
		`{"role":"user","type":"input_file","file_data":"data:text/plain;base64,b3JkaW5hcnk=","content":` + blocked + `}`,
	} {
		for _, field := range []string{"input", "INPUT", "messages", "MESSAGES"} {
			protocol := "openai_responses"
			if strings.EqualFold(field, "messages") {
				protocol = "openai_chat_completions"
			}
			req := Request{Protocol: protocol, Body: []byte(`{"TYPE":"input_file","file_data":"data:text/plain;base64,b3JkaW5hcnk=","` + field + `":[` + message + `]}`)}
			snapshot, err := ExtractPromptSnapshot(req)
			require.NoError(t, err)
			require.Equal(t, "blocked file", snapshot.ScanText)
		}
	}
	req := Request{Protocol: "openai_responses", Body: []byte(`{"input":[{"role":"user","content":` + ordinary + `}],"INPUT":[{"role":"user","content":` + blocked + `}]}`)}
	snapshot, err := ExtractPromptSnapshot(req)
	require.NoError(t, err)
	require.Equal(t, "blocked file", snapshot.ScanText)
}

func TestPromptSnapshotInlineFileChatMessageTypeExtension(t *testing.T) {
	for _, extension := range []string{`17`, `{}`, `[]`, `null`} {
		req := Request{Protocol: "openai_chat_completions", Body: []byte(`{"messages":[{"role":"user","type":` + extension + `,"content":[{"type":"file","file":{"file_data":"data:text/plain;base64,YmxvY2tlZCBmaWxl"}}]}]}`)}
		for _, latestOnly := range []bool{false, true} {
			snapshot, err := ExtractBlockingPromptSnapshot(req, latestOnly)
			require.NoError(t, err)
			require.Equal(t, "blocked file", snapshot.ScanText)
		}
	}
}

func TestPromptSnapshotHTTPTypeIsNotWebSocketFrameType(t *testing.T) {
	for _, kind := range []string{"file", "input_file", "response.cancel"} {
		for _, content := range []string{`"blocked ordinary text"`, `[{"type":"input_file","file_data":"data:text/plain;base64,YmxvY2tlZCBmaWxl"}]`} {
			req := Request{Protocol: "openai_responses", Body: []byte(`{"type":"` + kind + `","input":[{"role":"user","content":` + content + `}]}`)}
			snapshot, err := ExtractPromptSnapshot(req)
			require.NoError(t, err)
			require.Contains(t, snapshot.ScanText, "blocked")
			req.Protocol = "responses_websocket"
			_, err = ExtractPromptSnapshot(req)
			require.ErrorIs(t, err, ErrNoPromptText)
		}
	}
}

func TestPromptServiceBlocksInlineFileOnlyRequests(t *testing.T) {
	for _, protocol := range []string{"openai_chat_completions", "openai_responses"} {
		for _, latestOnly := range []bool{false, true} {
			seen := []string{}
			wantSeen := []string{}
			scanner := PromptScannerFunc(func(_ context.Context, _ ActiveEndpoint, text string, _ []string) (*NormalizedResult, error) {
				seen = append(seen, text)
				result := &NormalizedResult{Decision: EventPass, RiskLevel: RiskLow, Action: ActionAllow, ScannerScores: map[string]float64{}, ScannerEvidence: map[string]string{}}
				if strings.Contains(text, "POLICY_BLOCK") {
					result.Decision, result.RiskLevel, result.Action = EventCritical, RiskCritical, ActionBlock
				}
				return result, nil
			})
			svc := &PromptService{
				config: &fakeConfigStore{active: true, cfg: ActiveConfig{
					RiskControlEnabled: true, Enabled: true, BlockingEnabled: true, AllGroups: true, BlockingLatestTurnOnly: latestOnly,
					Scanners: AllScannerIDs, Endpoints: []ActiveEndpoint{{ID: "local-policy", Enabled: true, TimeoutMS: 1000, InputLimit: 4096}},
				}},
				evaluator: newGuardEvaluator(scanner, nil, NewAtomicMetrics(), 2, 2),
			}
			for _, text := range []string{"POLICY_BLOCK file", "ordinary file"} {
				kinds := []string{""}
				if protocol == "openai_responses" {
					kinds = append(kinds, "file", "input_file", "response.cancel")
				}
				for _, kind := range kinds {
					req := inlineFileRequest(t, protocol, text)
					if kind != "" {
						req.Body = []byte(`{"type":"` + kind + `",` + string(req.Body[1:]))
					}
					decision, err := svc.Evaluate(context.Background(), req)
					require.NoError(t, err)
					require.Equal(t, text == "ordinary file", decision.AllowNextStage)
					if text != "ordinary file" {
						require.Equal(t, DecisionBlock, decision.Kind)
					}
					wantSeen = append(wantSeen, text)
				}
			}
			require.Equal(t, wantSeen, seen)
		}
	}
}

func TestPromptEnqueuerRetainsInlineFileText(t *testing.T) {
	for _, protocol := range []string{"openai_chat_completions", "openai_responses"} {
		repo := &fakeJobRepository{createJob: &Job{ID: 41}}
		payload := &fakePayloadStore{values: map[int64]string{}}
		enqueuer := NewEnqueuer(&fakeConfigStore{cfg: asyncConfig(), active: true}, repo, payload)
		const text = "file prompt email@example.com sk-secretvalue123"
		require.NoError(t, enqueuer.Enqueue(context.Background(), inlineFileRequest(t, protocol, text)))
		require.Equal(t, text, payload.values[41])
		require.Equal(t, text, repo.createdSnapshot.FullPrompt)
		require.Equal(t, 1, repo.createdSnapshot.MessageCount)
		require.Empty(t, repo.createdSnapshot.ScanText)
		require.NotContains(t, repo.createdSnapshot.RedactedPreview, "email@example.com")
		require.NotContains(t, repo.createdSnapshot.RedactedPreview, "secretvalue123")
	}
}
