package researchgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fixture struct {
	g         *gateway
	f         *fakeUpstream
	b         binding
	key, path string
	server    *httptest.Server
}

const completedSSE = "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"model\":\"gpt-6.1-sol\",\"usage\":{\"input_tokens\":1,\"input_tokens_details\":{\"cached_tokens\":0},\"output_tokens\":1,\"output_tokens_details\":{\"reasoning_tokens\":0}}}}\n\n"

func setup(t *testing.T, handler http.Handler) *fixture {
	t.Helper()
	if handler == nil {
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.Copy(io.Discard, r.Body)
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, completedSSE)
		})
	}
	f, err := ownFake(handler)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.close)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	key := token()
	ops := map[string]string{}
	for i := 0; i < 8; i++ {
		effort := "medium"
		if i >= 4 {
			effort = "high"
		}
		ops[fmt.Sprintf("%064x", i+1)] = effort
	}
	b := binding{Source: SourceRevision, Model: "gpt-6.1-sol", Account: "owned-offline-fixture", Credential: digest([]byte(f.key)), Key: digest([]byte(key)), Endpoint: f.endpoint, Operations: ops, Expires: time.Now().Add(180 * time.Second).UnixMilli()}
	path := filepath.Join(dir, "ledger.sqlite")
	l, err := openLedger(path, b, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.db.Close() })
	g, err := newGateway(l, f, b, key)
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(g)
	t.Cleanup(s.Close)
	return &fixture{g, f, b, key, path, s}
}

func body(effort string) string {
	return fmt.Sprintf(`{"model":"gpt-6.1-sol","input":"offline","reasoning":{"effort":%q},"stream":true,"store":false,"max_output_tokens":512}`, effort)
}
func (f *fixture) request(op, data, path string) *http.Request {
	r, _ := http.NewRequest("POST", f.server.URL+path, strings.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+f.key)
	r.Header.Set("X-Research-Operation", op)
	return r
}
func send(t *testing.T, r *http.Request) (int, string, error) {
	t.Helper()
	resp, err := (&http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}}).Do(r)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), err
}
func assertDenied(t *testing.T, f *fixture, r *http.Request) {
	t.Helper()
	status, _, err := send(t, r)
	if err != nil || status != 403 || f.f.count.Load() != 0 {
		t.Fatalf("status=%d err=%v dispatches=%d", status, err, f.f.count.Load())
	}
}

func TestZeroOneEightAndNinth(t *testing.T) {
	f := setup(t, nil)
	if err := validateBody([]byte(body("medium")), "medium"); err != nil {
		t.Fatal("validation", err)
	}
	transformed, err := service.ResearchOAuthTransform([]byte(body("medium")))
	if err != nil || len(transformed) > wireRequestBytes {
		t.Fatalf("transform bytes=%d err=%v", len(transformed), err)
	}
	if f.f.count.Load() != 0 {
		t.Fatal("startup inference")
	}
	for i := 1; i <= 8; i++ {
		op := fmt.Sprintf("%064x", i)
		status, _, err := send(t, f.request(op, body(f.b.Operations[op]), "/v1/responses"))
		if err != nil || status != 200 || f.f.count.Load() != int64(i) {
			t.Fatalf("operation %d status=%d err=%v count=%d", i, status, err, f.f.count.Load())
		}
	}
	for _, op := range []string{fmt.Sprintf("%064x", 1), token()} {
		status, _, _ := send(t, f.request(op, body("medium"), "/v1/responses"))
		if status != 403 || f.f.count.Load() != 8 {
			t.Fatal("ninth dispatch")
		}
	}
	var count int
	if err := f.g.ledger.db.QueryRow("SELECT COUNT(*) FROM attempts").Scan(&count); err != nil || count != 8 {
		t.Fatalf("ledger %d %v", count, err)
	}
}

func TestAlternateRoutesAndControls(t *testing.T) {
	mutations := map[string]func(*http.Request){
		"no identity":        func(r *http.Request) { r.Header.Del("X-Research-Operation") },
		"unknown identity":   func(r *http.Request) { r.Header.Set("X-Research-Operation", token()) },
		"wrong key":          func(r *http.Request) { r.Header.Set("Authorization", "Bearer unrelated") },
		"duplicate key":      func(r *http.Request) { r.Header.Add("Authorization", r.Header.Get("Authorization")) },
		"duplicate identity": func(r *http.Request) { r.Header.Add("X-Research-Operation", r.Header.Get("X-Research-Operation")) },
		"provider route":     func(r *http.Request) { r.Header.Set("X-Provider", "anthropic") },
		"alternate account":  func(r *http.Request) { r.Header.Set("X-Account", "alternate") },
		"forwarding proxy":   func(r *http.Request) { r.Header.Set("X-Forwarded-Host", "outside.invalid") },
		"get":                func(r *http.Request) { r.Method = "GET" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			f := setup(t, nil)
			r := f.request(fmt.Sprintf("%064x", 1), body("medium"), "/v1/responses")
			mutate(r)
			assertDenied(t, f, r)
		})
	}
	for _, path := range []string{"/v1/chat/completions", "/v1/messages", "/v1/models", "/v1/responses?model=other", "/v1/responses/", "/v1/responses/compact", "/backend-api/codex/responses", "/warmup", "/prefetch", "/recovery", "/ws"} {
		t.Run(path, func(t *testing.T) {
			f := setup(t, nil)
			assertDenied(t, f, f.request(fmt.Sprintf("%064x", 1), body("medium"), path))
		})
	}
	for name, data := range map[string]string{
		"wrong model":         strings.Replace(body("medium"), "gpt-6.1-sol", "other", 1),
		"wrong effort":        body("high"),
		"fast":                strings.TrimSuffix(body("medium"), "}") + `,"service_tier":"priority"}`,
		"duplicate model":     strings.TrimSuffix(body("medium"), "}") + `,"model":"gpt-6.1-sol"}`,
		"duplicate effort":    strings.Replace(body("medium"), `"effort":"medium"`, `"effort":"high","effort":"medium"`, 1),
		"completion field":    strings.TrimSuffix(body("medium"), "}") + `,"max_completion_tokens":512}`,
		"native tools":        strings.TrimSuffix(body("medium"), "}") + `,"tools":[{"type":"web_search"}]}`,
		"nested native tools": strings.Replace(body("medium"), `"input":"offline"`, `"input":[{"type":"message","role":"user","content":"offline","additional_tools":[{"type":"web_search"}]}]`, 1),
		"remote media":        strings.Replace(body("medium"), `"input":"offline"`, `"input":[{"type":"input_image","image_url":"https://outside.invalid/image"}]`, 1),
		"native choice":       strings.TrimSuffix(body("medium"), "}") + `,"tool_choice":{"type":"web_search"}}`,
		"output too large":    strings.Replace(body("medium"), "512", "513", 1),
		"oversized":           strings.Replace(body("medium"), "offline", strings.Repeat("x", requestBytes), 1),
		"invalid json":        "{",
		"trailing json":       body("medium") + "{}",
	} {
		t.Run(name, func(t *testing.T) {
			f := setup(t, nil)
			assertDenied(t, f, f.request(fmt.Sprintf("%064x", 1), data, "/v1/responses"))
		})
	}
}

func TestOAuthOutputFieldsAndEffort(t *testing.T) {
	for _, i := range []int{1, 5} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			var seen map[string]any
			f := setup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
					t.Error(err)
				}
				fmt.Fprint(w, strings.Replace(completedSSE, `"output_tokens":1`, `"output_tokens":2048`, 1))
			}))
			op := fmt.Sprintf("%064x", i)
			status, data, err := send(t, f.request(op, body(f.b.Operations[op]), "/v1/responses"))
			if status != 200 || err != nil || !strings.Contains(data, "2048") {
				t.Fatal(status, data, err)
			}
			if _, ok := seen["max_output_tokens"]; ok {
				t.Fatal("deployed transform unexpectedly preserves cap")
			}
			if _, ok := seen["max_completion_tokens"]; ok {
				t.Fatal("completion cap")
			}
			if seen["model"] != "gpt-6.1-sol" || seen["reasoning"].(map[string]any)["effort"] != f.b.Operations[op] || seen["store"] != false || seen["stream"] != true {
				t.Fatal("wire control changed")
			}
		})
	}
}

func TestNoErrorRetryOrFallback(t *testing.T) {
	for _, status := range []int{400, 401, 403, 408, 429, 500, 502, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f := setup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, `{"error":{"param":"max_output_tokens","message":"retry with another account"}}`)
			}))
			op := fmt.Sprintf("%064x", 1)
			send(t, f.request(op, body("medium"), "/v1/responses"))
			send(t, f.request(op, body("medium"), "/v1/responses"))
			send(t, f.request(fmt.Sprintf("%064x", 2), body("medium"), "/v1/responses"))
			if f.f.count.Load() != 1 {
				t.Fatal("error retry/fallback")
			}
		})
	}
	t.Run("redirect", func(t *testing.T) {
		f := setup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://outside.invalid/inference", 307)
		}))
		status, _, _ := send(t, f.request(fmt.Sprintf("%064x", 1), body("medium"), "/v1/responses"))
		if status != 307 || f.f.count.Load() != 1 {
			t.Fatal(status)
		}
	})
}

func TestTerminalFailureStopsCampaign(t *testing.T) {
	for name, data := range map[string]string{
		"missing terminal":    "data: {\"type\":\"response.in_progress\"}\n\n",
		"failed terminal":     "data: {\"type\":\"response.failed\"}\n\n",
		"incomplete terminal": "data: {\"type\":\"response.incomplete\"}\n\n",
		"missing frame end":   strings.TrimSuffix(completedSSE, "\n\n"),
		"duplicate terminal":  completedSSE + completedSSE,
		"wrong model":         strings.Replace(completedSSE, "gpt-6.1-sol", "other", 1),
		"malformed usage":     strings.Replace(completedSSE, `"input_tokens":1`, `"input_tokens":-1`, 1),
		"missing usage":       strings.Replace(completedSSE, `"usage":`, `"omitted":`, 1),
		"excess cache":        strings.Replace(completedSSE, `"cached_tokens":0`, `"cached_tokens":2`, 1),
		"excess reasoning":    strings.Replace(completedSSE, `"reasoning_tokens":0`, `"reasoning_tokens":2`, 1),
		"duplicate usage key": strings.Replace(completedSSE, `"input_tokens":1`, `"input_tokens":1,"input_tokens":1`, 1),
		"sentinel first":      "data: [DONE]\n\n" + completedSSE,
		"raw json":            "{}",
		"non utf8":            string([]byte{255}) + completedSSE,
	} {
		t.Run(name, func(t *testing.T) {
			f := setup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, data) }))
			_, _, err := send(t, f.request(fmt.Sprintf("%064x", 1), body("medium"), "/v1/responses"))
			if err == nil {
				t.Fatal("invalid stream ended cleanly")
			}
			send(t, f.request(fmt.Sprintf("%064x", 2), body("medium"), "/v1/responses"))
			if f.f.count.Load() != 1 {
				t.Fatal("invalid stream continued")
			}
		})
	}
	t.Run("sentinel after completion", func(t *testing.T) {
		f := setup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, completedSSE+"data: [DONE]\n\n") }))
		status, _, err := send(t, f.request(fmt.Sprintf("%064x", 1), body("medium"), "/v1/responses"))
		if status != 200 || err != nil {
			t.Fatal(status, err)
		}
	})
}

func TestStreamingBoundsAndDisconnect(t *testing.T) {
	t.Run("exact byte ceiling", func(t *testing.T) {
		f := setup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, ":"+strings.Repeat("x", responseBytes-len(completedSSE)-3)+"\n\n"+completedSSE)
		}))
		status, data, err := send(t, f.request(fmt.Sprintf("%064x", 1), body("medium"), "/v1/responses"))
		if status != 200 || err != nil || len(data) != responseBytes {
			t.Fatal(status, len(data), err)
		}
	})
	t.Run("malformed usage remains untrusted", func(t *testing.T) {
		data := `data: {"type":"response.completed","response":{"usage":{"input_tokens":-1}}}` + "\n\n"
		f := setup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, data) }))
		status, received, err := send(t, f.request(fmt.Sprintf("%064x", 1), body("medium"), "/v1/responses"))
		if status != 200 || err == nil || received != data || f.f.count.Load() != 1 {
			t.Fatal(status, err)
		}
		send(t, f.request(fmt.Sprintf("%064x", 2), body("medium"), "/v1/responses"))
		if f.f.count.Load() != 1 {
			t.Fatal("malformed usage continued")
		}
	})
	t.Run("slow request body", func(t *testing.T) {
		f := setup(t, nil)
		f.g.timeout = 30 * time.Millisecond
		conn, err := net.Dial("tcp", strings.TrimPrefix(f.server.URL, "http://"))
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(time.Second))
		fmt.Fprintf(conn, "POST /v1/responses HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nAuthorization: Bearer %s\r\nX-Research-Operation: %064x\r\nContent-Length: 100\r\n\r\n{", f.key, 1)
		b := make([]byte, 1024)
		n, err := conn.Read(b)
		if err != nil || !strings.Contains(string(b[:n]), "403") || f.f.count.Load() != 0 {
			t.Fatal(string(b[:n]), err)
		}
	})
	t.Run("downstream early disconnect", func(t *testing.T) {
		cancelled := make(chan struct{})
		f := setup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.Copy(io.Discard, r.Body)
			fmt.Fprint(w, "data: first\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			close(cancelled)
		}))
		r := f.request(fmt.Sprintf("%064x", 1), body("medium"), "/v1/responses")
		client := &http.Client{Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}}
		resp, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		b := make([]byte, 13)
		if _, err = io.ReadFull(resp.Body, b); err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		select {
		case <-cancelled:
		case <-time.After(time.Second):
			t.Fatal("upstream cancellation lost")
		}
		send(t, f.request(fmt.Sprintf("%064x", 2), body("medium"), "/v1/responses"))
		if f.f.count.Load() != 1 {
			t.Fatal("disconnect retry")
		}
	})
	t.Run("flush", func(t *testing.T) {
		release := make(chan struct{})
		defer close(release)
		f := setup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "data: first\n\n")
			w.(http.Flusher).Flush()
			select {
			case <-release:
			case <-r.Context().Done():
			}
			fmt.Fprint(w, "data: last\n\n")
		}))
		r := f.request(fmt.Sprintf("%064x", 1), body("medium"), "/v1/responses")
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b := make([]byte, len("data: first\n\n"))
		if _, err = io.ReadFull(resp.Body, b); err != nil || string(b) != "data: first\n\n" {
			t.Fatal(string(b), err)
		}
	})
	t.Run("response ceiling", func(t *testing.T) {
		f := setup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, strings.Repeat("x", responseBytes+1)) }))
		_, data, err := send(t, f.request(fmt.Sprintf("%064x", 1), body("medium"), "/v1/responses"))
		if err == nil || len(data) > responseBytes || f.f.count.Load() != 1 {
			t.Fatal(len(data), err)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		f := setup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		f.g.timeout = 30 * time.Millisecond
		status, _, _ := send(t, f.request(fmt.Sprintf("%064x", 1), body("medium"), "/v1/responses"))
		if status != 502 || f.f.count.Load() != 1 {
			t.Fatal(status)
		}
		send(t, f.request(fmt.Sprintf("%064x", 2), body("medium"), "/v1/responses"))
		if f.f.count.Load() != 1 {
			t.Fatal("timeout retry")
		}
	})
	t.Run("upstream early disconnect", func(t *testing.T) {
		f := setup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { conn, _, _ := w.(http.Hijacker).Hijack(); conn.Close() }))
		status, _, _ := send(t, f.request(fmt.Sprintf("%064x", 1), body("medium"), "/v1/responses"))
		if status != 502 || f.f.count.Load() != 1 {
			t.Fatal(status)
		}
	})
	t.Run("cancel before reservation", func(t *testing.T) {
		f := setup(t, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		r := f.request(fmt.Sprintf("%064x", 1), body("medium"), "/v1/responses").WithContext(ctx)
		send(t, r)
		if f.f.count.Load() != 0 {
			t.Fatal("cancelled dispatch")
		}
	})
}

func TestLedgerFailuresRevocationAndBinding(t *testing.T) {
	t.Run("database transaction lock", func(t *testing.T) {
		f := setup(t, nil)
		other, err := openLedger(f.path, f.b, false)
		if err != nil {
			t.Fatal(err)
		}
		defer other.db.Close()
		tx, err := other.db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		f.g.timeout = 30 * time.Millisecond
		assertDenied(t, f, f.request(fmt.Sprintf("%064x", 1), body("medium"), "/v1/responses"))
	})
	t.Run("settlement unavailable", func(t *testing.T) {
		release := make(chan struct{})
		started := make(chan struct{})
		f := setup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			<-release
			fmt.Fprint(w, "data: complete\n\n")
		}))
		done := make(chan error, 1)
		go func() {
			_, _, err := send(t, f.request(fmt.Sprintf("%064x", 1), body("medium"), "/v1/responses"))
			done <- err
		}()
		<-started
		f.g.ledger.db.Close()
		close(release)
		if <-done == nil {
			t.Fatal("settlement failure appeared successful")
		}
		l, err := openLedger(f.path, f.b, false)
		if err != nil {
			t.Fatal(err)
		}
		defer l.db.Close()
		if l.reserve(context.Background(), fmt.Sprintf("%064x", 2), digest([]byte("next"))) == nil {
			t.Fatal("pending reservation ignored")
		}
	})
	t.Run("redis and postgres absent", func(t *testing.T) {
		t.Setenv("REDIS_URL", "redis://127.0.0.1:1")
		t.Setenv("DATABASE_URL", "postgres://127.0.0.1:1/unavailable")
		f := setup(t, nil)
		status, _, err := send(t, f.request(fmt.Sprintf("%064x", 1), body("medium"), "/v1/responses"))
		if status != 200 || err != nil || f.f.count.Load() != 1 {
			t.Fatal(status, err)
		}
	})
	for name, change := range map[string]func(*fixture){
		"database closed": func(f *fixture) { f.g.ledger.db.Close() },
		"database absent": func(f *fixture) { os.Remove(f.path) },
		"revoked": func(f *fixture) {
			if _, err := f.g.ledger.db.Exec("UPDATE campaign SET revoked=1"); err != nil {
				panic(err)
			}
		},
		"binding changed": func(f *fixture) {
			if _, err := f.g.ledger.db.Exec("UPDATE campaign SET identity='changed'"); err != nil {
				panic(err)
			}
		},
		"credential changed": func(f *fixture) { f.f.key = token() },
		"endpoint changed":   func(f *fixture) { f.f.endpoint = "http://127.0.0.1:1/other" },
		"expired":            func(f *fixture) { f.g.bound.Expires = time.Now().Add(-time.Second).UnixMilli() },
		"monotonic expiry":   func(f *fixture) { f.g.started = time.Now().Add(-181 * time.Second) },
		"unsafe permissions": func(f *fixture) { os.Chmod(f.path, 0644) },
	} {
		t.Run(name, func(t *testing.T) {
			f := setup(t, nil)
			change(f)
			assertDenied(t, f, f.request(fmt.Sprintf("%064x", 1), body("medium"), "/v1/responses"))
		})
	}
	t.Run("changed constructor binding", func(t *testing.T) {
		f := setup(t, nil)
		b := f.b
		b.Operations = map[string]string{}
		for k, v := range f.b.Operations {
			b.Operations[k] = v
		}
		b.Operations[token()] = "medium"
		if _, err := newGateway(f.g.ledger, f.f, b, f.key); err == nil {
			t.Fatal("changed binding")
		}
	})
	t.Run("caller map mutation", func(t *testing.T) {
		f := setup(t, nil)
		id := token()
		f.b.Operations[id] = "medium"
		assertDenied(t, f, f.request(id, body("medium"), "/v1/responses"))
	})
	t.Run("missing database reopen", func(t *testing.T) {
		f := setup(t, nil)
		f.g.ledger.db.Close()
		os.Remove(f.path)
		if _, err := openLedger(f.path, f.b, false); err == nil {
			t.Fatal("recreated missing database")
		}
	})
}

func TestProcessHelper(t *testing.T) {
	if os.Getenv("RESEARCH_HELPER") != "1" {
		return
	}
	var b binding
	if err := json.Unmarshal([]byte(os.Getenv("RESEARCH_BINDING")), &b); err != nil {
		t.Fatal(err)
	}
	l, err := openLedger(os.Getenv("RESEARCH_LEDGER"), b, false)
	if err != nil {
		t.Fatal(err)
	}
	defer l.db.Close()
	op := os.Getenv("RESEARCH_OPERATION")
	// The parent owns the fake destination. Only this test helper reconstructs
	// its synthetic binding so separate processes exercise the real handler.
	f := &fakeUpstream{endpoint: b.Endpoint, key: os.Getenv("RESEARCH_FAKE_KEY")}
	key := os.Getenv("RESEARCH_DOWNSTREAM_KEY")
	g, err := newGateway(l, f, b, key)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("RESEARCH_CRASH") == "before" {
		g.transport.DialContext = func(context.Context, string, string) (net.Conn, error) { fmt.Println("RESERVED"); select {} }
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("RESEARCH_CRASH") == "after" {
			g.ServeHTTP(crashWriter{w}, r)
		} else {
			g.ServeHTTP(w, r)
		}
	}))
	defer s.Close()
	r, err := http.NewRequest("POST", s.URL+"/v1/responses", strings.NewReader(body(b.Operations[op])))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+key)
	r.Header.Set("X-Research-Operation", op)
	status, _, err := send(t, r)
	if err != nil || status != 200 {
		os.Exit(3)
	}
}

type crashWriter struct{ http.ResponseWriter }

func (w crashWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w crashWriter) Write([]byte) (int, error) { fmt.Println("DISPATCHED"); select {} }

func process(f *fixture, op, crash string) *exec.Cmd {
	b, _ := json.Marshal(f.b)
	cmd := exec.Command(os.Args[0], "-test.run=^TestProcessHelper$")
	cmd.Env = append(os.Environ(), "RESEARCH_HELPER=1", "RESEARCH_BINDING="+string(b), "RESEARCH_LEDGER="+f.path, "RESEARCH_OPERATION="+op, "RESEARCH_CRASH="+crash, "RESEARCH_FAKE_KEY="+f.f.key, "RESEARCH_DOWNSTREAM_KEY="+f.key)
	return cmd
}

func TestCrashRestartBeforeAndAfterDispatch(t *testing.T) {
	for _, phase := range []string{"before", "after"} {
		t.Run(phase, func(t *testing.T) {
			f := setup(t, nil)
			op := fmt.Sprintf("%064x", 1)
			cmd := process(f, op, phase)
			out, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			line := make([]byte, 1)
			var text strings.Builder
			for !strings.Contains(text.String(), "\n") {
				if _, err = out.Read(line); err != nil {
					cmd.Process.Kill()
					cmd.Wait()
					t.Fatal(err)
				}
				text.Write(line)
			}
			if err = cmd.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			cmd.Wait()
			l, err := openLedger(f.path, f.b, false)
			if err != nil {
				t.Fatal(err)
			}
			defer l.db.Close()
			if err = l.reserve(context.Background(), op, digest([]byte("retry"))); err == nil {
				t.Fatal("crash replay")
			}
			if err = l.reserve(context.Background(), fmt.Sprintf("%064x", 2), digest([]byte("next"))); err == nil {
				t.Fatal("uncertain campaign continued")
			}
			want := int64(0)
			if phase == "after" {
				want = 1
			}
			if f.f.count.Load() != want {
				t.Fatal(f.f.count.Load())
			}
		})
	}
}

func TestConcurrentWorkersAndProcessCeiling(t *testing.T) {
	f := setup(t, nil)
	l2, err := openLedger(f.path, f.b, false)
	if err != nil {
		t.Fatal(err)
	}
	defer l2.db.Close()
	g2, err := newGateway(l2, f.f, f.b, f.key)
	if err != nil {
		t.Fatal(err)
	}
	s2 := httptest.NewServer(g2)
	defer s2.Close()
	for i := 1; i <= 8; i++ {
		op := fmt.Sprintf("%064x", i)
		var wg sync.WaitGroup
		for j := 0; j < 8; j++ {
			wg.Add(1)
			go func(j int) {
				defer wg.Done()
				r := f.request(op, body(f.b.Operations[op]), "/v1/responses")
				if j%2 == 1 {
					u, _ := url.Parse(s2.URL + "/v1/responses")
					r.URL = u
				}
				send(t, r)
			}(j)
		}
		wg.Wait()
		if f.f.count.Load() != int64(i) {
			t.Fatalf("workers dispatched %d at %d", f.f.count.Load(), i)
		}
	}
	for i := 0; i < 8; i++ {
		if err := process(f, fmt.Sprintf("%064x", i+1), "").Run(); err == nil {
			t.Fatal("process exceeded eight")
		}
	}
}

func TestConcurrentSeparateProcesses(t *testing.T) {
	f := setup(t, nil)
	for i := 1; i <= 8; i++ {
		op := fmt.Sprintf("%064x", i)
		cmds := []*exec.Cmd{}
		for j := 0; j < 4; j++ {
			cmd := process(f, op, "")
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			cmds = append(cmds, cmd)
		}
		accepted := 0
		for _, cmd := range cmds {
			if cmd.Wait() == nil {
				accepted++
			}
		}
		if accepted != 1 || f.f.count.Load() != int64(i) {
			t.Fatalf("process successes=%d count=%d op=%d", accepted, f.f.count.Load(), i)
		}
	}
	if process(f, fmt.Sprintf("%064x", 1), "").Run() == nil || f.f.count.Load() != 8 {
		t.Fatal("process ninth dispatch")
	}
}

func TestNoOutsideEgress(t *testing.T) {
	if os.Getenv("RESEARCH_OS_SANDBOX") != "1" {
		t.Fatal("tests require an OS egress sandbox")
	}
	conn, err := net.DialTimeout("tcp", "192.0.2.1:80", 100*time.Millisecond)
	if conn != nil {
		conn.Close()
	}
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "operation not permitted") {
		t.Fatalf("outside egress not demonstrably denied: %v", err)
	}
	f := setup(t, nil)
	if _, err = f.g.transport.DialContext(context.Background(), "tcp", "192.0.2.1:80"); err == nil {
		t.Fatal("dispatch dial bypass")
	}
}

func TestOfflineEntryPoint(t *testing.T) {
	if err := RunOffline(); err != nil {
		t.Fatal(err)
	}
}
