package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
)

// ForwardAsResponses uses native generateContent with stateless full-history
// input. OAuth/subscription accounts remain outside this API-key adapter.
func (s *GeminiMessagesCompatService) ForwardAsResponses(ctx context.Context, c *gin.Context, account *Account, body []byte) (*ForwardResult, error) {
	if account.Type != AccountTypeAPIKey {
		return nil, writeNativeGeminiResponsesError(c, http.StatusBadRequest, "invalid_request_error", "native Gemini Responses requires an API-key account")
	}
	start := time.Now()
	observer := beginUpstreamResponseModelObservation(c)
	adapted, clientTools, err := adaptNativeGeminiNamespaces(body)
	if err != nil {
		return nil, writeNativeGeminiResponsesError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
	}
	payload, req, err := responsesToNativeGemini(adapted)
	if err != nil {
		return nil, writeNativeGeminiResponsesError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
	}
	mapped := account.GetMappedModel(req.Model)
	build, _ := s.buildGeminiChatCompletionsUpstreamRequestFunc(account, mapped, payload, req.Stream, req.Stream)
	upstream, header, err := build(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, writeNativeGeminiResponsesError(c, http.StatusBadGateway, "upstream_error", sanitizeUpstreamErrorMessage(err.Error()))
	}
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(upstream, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return nil, s.handleUpstreamTransportError(ctx, c, account, err)
	}
	defer func() { _ = resp.Body.Close() }()
	requestID := resp.Header.Get(header)
	if requestID == "" {
		requestID = resp.Header.Get("x-goog-request-id")
	}
	if requestID != "" {
		c.Header("x-request-id", requestID)
	}
	if resp.StatusCode >= 400 {
		raw := s.readUpstreamErrorBody(resp)
		policy := ErrorPolicyNone
		if s.rateLimitService != nil {
			policy = s.rateLimitService.CheckErrorPolicy(ctx, account, resp.StatusCode, raw, mapped)
		}
		if policy == ErrorPolicyNone || policy == ErrorPolicyMatched {
			s.handleGeminiUpstreamError(ctx, account, resp.StatusCode, resp.Header, raw)
		}
		if s.shouldFailoverGeminiUpstreamError(resp.StatusCode) {
			return nil, &UpstreamFailoverError{StatusCode: resp.StatusCode, ResponseBody: raw, RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode)}
		}
		if policy == ErrorPolicySkipped && account.IsCustomErrorCodesEnabled() {
			return nil, s.writeGeminiCustomCodeSkippedError(c, account, resp.StatusCode, requestID, raw, func() {
				_ = writeNativeGeminiResponsesError(c, http.StatusInternalServerError, "api_error", geminiCustomCodeSkippedClientMessage)
			})
		}
		if c.Writer.Written() {
			return nil, writeNativeGeminiResponsesError(c, mapUpstreamStatusCode(resp.StatusCode), "upstream_error", "Upstream request failed")
		}
		return nil, s.writeGeminiChatCompletionsMappedError(c, account, resp.StatusCode, requestID, raw)
	}
	responseID := "resp_" + randomHex(16)
	var response *apicompat.ResponsesResponse
	var firstTokenMs *int
	if req.Stream {
		response, firstTokenMs, err = streamNativeGeminiResponses(ctx, c, resp.Body, req.Model, responseID, start, clientTools)
	} else {
		var raw []byte
		raw, err = ReadUpstreamResponseBody(resp.Body, s.cfg, c, func(c *gin.Context) {
			_ = writeNativeGeminiResponsesError(c, http.StatusBadGateway, "upstream_error", "Upstream response too large")
		})
		if err == nil {
			observer.ObserveGemini(raw)
			response, err = nativeGeminiResponse(raw, req.Model, responseID)
			if err == nil {
				nativeGeminiRequireOutput(response)
			} else {
				var body map[string]any
				if decodeOpenAIJSONUseNumber(raw, &body) == nil {
					if usage := nativeGeminiResponsesUsage(body); usage != nil {
						response = &apicompat.ResponsesResponse{ID: responseID, Object: "response", Model: req.Model, Status: "failed", Output: []apicompat.ResponsesOutput{}, Usage: usage,
							Error: &apicompat.ResponsesError{Code: "upstream_error", Message: "Invalid Gemini response"}}
						c.JSON(http.StatusBadGateway, response)
						MarkResponseCommitted(c)
					}
				}
			}
		}
		if err == nil {
			raw, err = json.Marshal(response)
			if err == nil {
				raw, _, err = apicompat.RestoreResponsesClientToolPayload(raw, clientTools)
			}
			if err == nil {
				c.Data(http.StatusOK, "application/json", raw)
			}
		}
		if err != nil && !IsResponseCommitted(c) {
			_ = writeNativeGeminiResponsesError(c, http.StatusBadGateway, "upstream_error", "Invalid Gemini response")
		}
	}
	clientDisconnect := err != nil && response != nil && (ctx.Err() != nil || c.Request.Context().Err() != nil || response.Status == "cancelled")
	if err != nil && !clientDisconnect {
		if response == nil || response.Status != "failed" || !IsResponseCommitted(c) {
			return nil, err
		}
	}
	if response.Status == "failed" {
		setOpsUpstreamError(c, http.StatusBadGateway, "Gemini generation failed", "")
	}
	usage := ClaudeUsage{}
	if response.Usage != nil {
		usage.InputTokens = response.Usage.InputTokens
		usage.OutputTokens = response.Usage.OutputTokens
		if response.Usage.InputTokensDetails != nil {
			usage.CacheReadInputTokens = response.Usage.InputTokensDetails.CachedTokens
			usage.InputTokens = max(usage.InputTokens-usage.CacheReadInputTokens, 0)
		}
	}
	return &ForwardResult{RequestID: requestID, UpstreamHeaders: resp.Header, Model: req.Model, UpstreamModel: mapped, UpstreamResponseModel: observedUpstreamResponseModel(c), UpstreamResponseModelConflict: observedUpstreamResponseModelConflict(c), Usage: usage, Stream: req.Stream, Duration: time.Since(start), FirstTokenMs: firstTokenMs, ClientDisconnect: clientDisconnect, ResponseStatus: response.Status}, nil
}

func streamNativeGeminiResponses(ctx context.Context, c *gin.Context, body io.Reader, model, id string, start time.Time, clientTools apicompat.ResponsesClientToolMapping) (result *apicompat.ResponsesResponse, firstToken *int, streamErr error) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	response := &apicompat.ResponsesResponse{ID: id, Object: "response", CreatedAt: time.Now().Unix(), Model: model, Status: "in_progress", Output: []apicompat.ResponsesOutput{}}
	observer := upstreamResponseModelObserverFromContext(c)
	if observer == nil {
		observer = beginUpstreamResponseModelObservation(c)
	}
	writeFailed := false
	defer func() {
		if writeFailed {
			response.Status = "cancelled"
			result = response
		}
	}()
	sequence := 0
	restorer := apicompat.NewResponsesClientToolStreamRestorer(clientTools)
	emit := func(event apicompat.ResponsesStreamEvent) error {
		event.SequenceNumber = sequence
		sequence++
		// Restoration mutates items; keep canonical names for terminal events and replay.
		if event.Item != nil {
			item := *event.Item
			event.Item = &item
		}
		if event.Response != nil {
			response := *event.Response
			response.Output = append([]apicompat.ResponsesOutput(nil), event.Response.Output...)
			event.Response = &response
		}
		for _, restored := range restorer.Restore(event) {
			data, err := json.Marshal(restored)
			if err != nil {
				return err
			}
			if _, err = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", restored.Type, data); err != nil {
				writeFailed = true
				return err
			}
			c.Writer.Flush()
		}
		return nil
	}
	if err := emit(apicompat.ResponsesStreamEvent{Type: "response.created", Response: response}); err != nil {
		return nil, nil, err
	}
	if err := emit(apicompat.ResponsesStreamEvent{Type: "response.in_progress", Response: response}); err != nil {
		return nil, nil, err
	}
	var first *int
	fail := func(err error) (*apicompat.ResponsesResponse, *int, error) {
		if ctx.Err() != nil {
			response.Status = "cancelled"
			return response, first, ctx.Err()
		}
		response.Status = "failed"
		response.Error = &apicompat.ResponsesError{Code: "upstream_error", Message: "Gemini stream failed before completion"}
		if writeErr := emit(apicompat.ResponsesStreamEvent{Type: "response.failed", Response: response}); writeErr != nil {
			return nil, first, writeErr
		}
		MarkResponseCommitted(c)
		return response, first, err
	}
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 4096), 16*1024*1024)
	finished := false
	chunkIndex := 0
	seenCalls := map[string]bool{}
	for scanner.Scan() {
		if ctx.Err() != nil {
			response.Status = "cancelled"
			return response, first, ctx.Err()
		}
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}
		var raw map[string]any
		if decodeOpenAIJSONUseNumber([]byte(data), &raw) != nil {
			return fail(errors.New("invalid Gemini SSE payload"))
		}
		observer.ObserveGemini([]byte(data))
		if usage := nativeGeminiResponsesUsage(raw); usage != nil {
			response.Usage = usage
		}
		if raw["error"] != nil {
			return fail(errors.New("gemini SSE upstream error"))
		}
		if finished && len(extractGeminiParts(raw)) > 0 {
			return fail(errors.New("gemini output after termination"))
		}
		chunk, err := nativeGeminiResponse([]byte(data), model, fmt.Sprintf("%s_chunk_%d", id, chunkIndex))
		chunkIndex++
		if err != nil {
			return fail(err)
		}
		if len(chunk.Output) > 0 && first == nil {
			elapsed := int(time.Since(start).Milliseconds())
			first = &elapsed
		}
		for _, item := range chunk.Output {
			if item.Type == "function_call" {
				if seenCalls[item.CallID] {
					return fail(errors.New("duplicate Gemini function call ID"))
				}
				seenCalls[item.CallID] = true
			}
			index := len(response.Output)
			added := item
			added.Status = "in_progress"
			if added.Type == "message" {
				added.Content = []apicompat.ResponsesContentPart{}
			}
			if added.Type == "function_call" {
				added.Arguments = ""
			}
			if err := emit(apicompat.ResponsesStreamEvent{Type: "response.output_item.added", OutputIndex: index, Item: &added}); err != nil {
				return nil, first, err
			}
			switch item.Type {
			case "message":
				part := apicompat.ResponsesContentPart{Type: "output_text"}
				if err := emit(apicompat.ResponsesStreamEvent{Type: "response.content_part.added", OutputIndex: index, ItemID: item.ID, Part: &part}); err != nil {
					return nil, first, err
				}
				text := item.Content[0].Text
				if err := emit(apicompat.ResponsesStreamEvent{Type: "response.output_text.delta", OutputIndex: index, ItemID: item.ID, Delta: text}); err != nil {
					return nil, first, err
				}
				if err := emit(apicompat.ResponsesStreamEvent{Type: "response.output_text.done", OutputIndex: index, ItemID: item.ID, Text: text}); err != nil {
					return nil, first, err
				}
				part = item.Content[0]
				if err := emit(apicompat.ResponsesStreamEvent{Type: "response.content_part.done", OutputIndex: index, ItemID: item.ID, Part: &part}); err != nil {
					return nil, first, err
				}
			case "function_call":
				if err := emit(apicompat.ResponsesStreamEvent{Type: "response.function_call_arguments.delta", OutputIndex: index, ItemID: item.ID, Delta: item.Arguments}); err != nil {
					return nil, first, err
				}
				if err := emit(apicompat.ResponsesStreamEvent{Type: "response.function_call_arguments.done", OutputIndex: index, ItemID: item.ID, Arguments: item.Arguments}); err != nil {
					return nil, first, err
				}
			}
			if err := emit(apicompat.ResponsesStreamEvent{Type: "response.output_item.done", OutputIndex: index, Item: &item}); err != nil {
				return nil, first, err
			}
			response.Output = append(response.Output, item)
		}
		if extractGeminiFinishReason(raw) != "" || chunk.IncompleteDetails != nil {
			finished = true
			response.Status = chunk.Status
			response.IncompleteDetails = chunk.IncompleteDetails
			response.Error = chunk.Error
		}
	}
	if scanner.Err() != nil {
		return fail(scanner.Err())
	}
	if !finished {
		return fail(io.ErrUnexpectedEOF)
	}
	if ctx.Err() != nil {
		response.Status = "cancelled"
		return response, first, ctx.Err()
	}
	nativeGeminiRequireOutput(response)
	if err := emit(apicompat.ResponsesStreamEvent{Type: "response." + response.Status, Response: response}); err != nil {
		return nil, first, err
	}
	MarkResponseCommitted(c)
	if response.Status == "failed" {
		return response, first, errors.New("gemini generation failed")
	}
	return response, first, nil
}

func writeNativeGeminiResponsesError(c *gin.Context, status int, code, message string) error {
	if !c.Writer.Written() {
		writeResponsesError(c, status, code, message)
	} else if !IsResponseCommitted(c) {
		response := &apicompat.ResponsesResponse{ID: "resp_" + randomHex(16), Object: "response", CreatedAt: time.Now().Unix(), Status: "failed", Output: []apicompat.ResponsesOutput{}, Error: &apicompat.ResponsesError{Code: code, Message: message}}
		data, _ := json.Marshal(apicompat.ResponsesStreamEvent{Type: "response.failed", Response: response})
		_, _ = fmt.Fprintf(c.Writer, "event: response.failed\ndata: %s\n\n", data)
		c.Writer.Flush()
		MarkResponseCommitted(c)
	}
	return errors.New(message)
}

func nativeGeminiRequireOutput(response *apicompat.ResponsesResponse) {
	if response.Status != "completed" {
		return
	}
	for _, item := range response.Output {
		if item.Type == "function_call" {
			return
		}
		for _, content := range item.Content {
			if item.Type == "message" && content.Text != "" {
				return
			}
		}
	}
	response.Status = "failed"
	response.Error = &apicompat.ResponsesError{Code: "upstream_error", Message: "Gemini response omitted output"}
}
