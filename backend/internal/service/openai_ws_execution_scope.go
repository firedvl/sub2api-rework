package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	openAIWSThreadIDHeader = "thread-id"
	openAIWSWindowIDHeader = "x-codex-window-id"

	openAIWSRequestKindTurn       = "turn"
	openAIWSRequestKindPrewarm    = "prewarm"
	openAIWSRequestKindCompaction = "compaction"
)

func resolveOpenAIWSClientThreadID(requestContext *gin.Context, body []byte) string {
	if requestContext != nil && requestContext.Request != nil {
		if id := strings.TrimSpace(requestContext.GetHeader(openAIWSThreadIDHeader)); id != "" {
			return id
		}
		if id := codexTurnMetadataThreadID(requestContext.GetHeader(openAIWSTurnMetadataHeader)); id != "" {
			return id
		}
		if window := strings.TrimSpace(requestContext.GetHeader(openAIWSWindowIDHeader)); window != "" {
			if id := strings.TrimSpace(strings.SplitN(window, ":", 2)[0]); id != "" {
				return id
			}
		}
	}
	if len(body) == 0 {
		return ""
	}
	if id := strings.TrimSpace(gjson.GetBytes(body, "client_metadata.thread_id").String()); id != "" {
		return id
	}
	return codexTurnMetadataThreadID(gjson.GetBytes(body, "client_metadata."+openAIWSTurnMetadataHeader).String())
}

type codexTurnMetadata struct {
	ThreadID    string `json:"thread_id"`
	RequestKind string `json:"request_kind"`
}

func parseCodexTurnMetadata(raw string) (codexTurnMetadata, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return codexTurnMetadata{}, false
	}
	var metadata codexTurnMetadata
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return codexTurnMetadata{}, false
	}
	metadata.ThreadID = strings.TrimSpace(metadata.ThreadID)
	metadata.RequestKind = strings.ToLower(strings.TrimSpace(metadata.RequestKind))
	return metadata, true
}

func codexTurnMetadataThreadID(raw string) string {
	metadata, _ := parseCodexTurnMetadata(raw)
	return metadata.ThreadID
}

func openAIWSExecutionTurnMetadata(requestContext *gin.Context, body []byte) codexTurnMetadata {
	if requestContext != nil && requestContext.Request != nil {
		if metadata, ok := parseCodexTurnMetadata(requestContext.GetHeader(openAIWSTurnMetadataHeader)); ok {
			return metadata
		}
	}
	if len(body) == 0 {
		return codexTurnMetadata{}
	}
	metadata, _ := parseCodexTurnMetadata(gjson.GetBytes(body, "client_metadata."+openAIWSTurnMetadataHeader).String())
	return metadata
}

func openAIWSExecutionSubagent(requestContext *gin.Context, body []byte) string {
	if requestContext != nil && requestContext.Request != nil {
		if subagent := strings.TrimSpace(requestContext.GetHeader(openAISubagentHeader)); subagent != "" {
			return strings.ToLower(subagent)
		}
	}
	if len(body) == 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "client_metadata."+openAISubagentHeader).String()))
}

func resolveOpenAIWSExecutionLane(requestContext *gin.Context, body []byte) string {
	metadata := openAIWSExecutionTurnMetadata(requestContext, body)
	switch metadata.RequestKind {
	case "", openAIWSRequestKindTurn, openAIWSRequestKindPrewarm, openAIWSRequestKindCompaction:
	default:
		return "kind=" + metadata.RequestKind
	}
	if metadata.ThreadID == "" {
		if subagent := openAIWSExecutionSubagent(requestContext, body); subagent != "" {
			return "subagent=" + subagent
		}
	}
	return ""
}

func openAIWSExecutionScopeSeed(apiKeyID int64, identity, value, lane string) string {
	seed := fmt.Sprintf("openai_ws_exec:%d|%s=%s", apiKeyID, identity, value)
	if lane != "" {
		seed += "|" + lane
	}
	return seed
}

func resolveOpenAIWSExecutionScope(requestContext *gin.Context, body []byte, apiKeyID int64) (scope, threadID string) {
	lane := resolveOpenAIWSExecutionLane(requestContext, body)
	if threadID = resolveOpenAIWSClientThreadID(requestContext, body); threadID != "" {
		scope, _ = deriveOpenAISessionHashes(openAIWSExecutionScopeSeed(apiKeyID, "thread", threadID, lane))
		return scope, threadID
	}
	if sessionID := strings.TrimSpace(explicitOpenAIRequestSessionID(requestContext, body)); sessionID != "" {
		scope, _ = deriveOpenAISessionHashes(openAIWSExecutionScopeSeed(apiKeyID, "session", sessionID, lane))
		return scope, ""
	}
	return "", ""
}
