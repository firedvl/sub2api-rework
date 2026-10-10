// Package researchgateway is an offline-only execution boundary. Its sole
// destination is a fake server created in this process; no live adapter exists.
package researchgateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "modernc.org/sqlite"
)

const SourceRevision = "c3b8715c497bb54c8f18ce4e2fe24b644f576aa8"
const requestBytes = 16 << 10
const wireRequestBytes = 64 << 10
const responseBytes = 256 << 10
const operationTimeout = 20 * time.Second

var denied = errors.New("research dispatch denied")

type binding struct {
	Source, Model, Account, Credential, Key, Endpoint string
	Operations                                        map[string]string
	Expires                                           int64
}

func digest(value []byte) string {
	h := sha256.Sum256(value)
	return hex.EncodeToString(h[:])
}

func token() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func privatePath(path string, directory bool) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for p := abs; ; p = filepath.Dir(p) {
		st, err := os.Lstat(p)
		if err != nil || st.Mode()&os.ModeSymlink != 0 {
			return denied
		}
		if p == abs {
			stat, ok := st.Sys().(*syscall.Stat_t)
			if !ok || int(stat.Uid) != os.Getuid() || st.Mode().Perm()&0077 != 0 || st.IsDir() != directory {
				return denied
			}
			if !directory && (!st.Mode().IsRegular() || stat.Nlink != 1) {
				return denied
			}
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	return nil
}

type ledger struct {
	db             *sql.DB
	path, identity string
}

func openLedger(path string, b binding, create bool) (*ledger, error) {
	if b.Source != SourceRevision || b.Model != "gpt-6.1-sol" || len(b.Operations) != 8 || b.Expires <= time.Now().UnixMilli() {
		return nil, denied
	}
	for id, effort := range b.Operations {
		if len(id) != 64 || (effort != "medium" && effort != "high") {
			return nil, denied
		}
		if _, err := hex.DecodeString(id); err != nil {
			return nil, denied
		}
	}
	if err := privatePath(filepath.Dir(path), true); err != nil {
		return nil, err
	}
	if create {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		err = f.Sync()
		closeErr := f.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	if err := privatePath(path, false); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: path}
	q := url.Values{"mode": {"rw"}, "_txlock": {"immediate"}, "_pragma": {"journal_mode(DELETE)", "synchronous(FULL)", "busy_timeout(25)"}}
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	data, err := json.Marshal(b)
	if err != nil {
		db.Close()
		return nil, err
	}
	l := &ledger{db: db, path: path, identity: digest(data)}
	if create {
		_, err = db.Exec(`CREATE TABLE campaign (identity TEXT PRIMARY KEY, revoked INTEGER NOT NULL DEFAULT 0, failed INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE attempts (operation TEXT PRIMARY KEY, body_digest TEXT NOT NULL, state TEXT NOT NULL CHECK(state IN ('reserved','finished')))`)
		if err == nil {
			_, err = db.Exec("INSERT INTO campaign(identity) VALUES (?)", l.identity)
		}
		if err == nil {
			var dir *os.File
			dir, err = os.Open(filepath.Dir(path))
			if err == nil {
				err = dir.Sync()
				dir.Close()
			}
		}
	}
	if err == nil {
		var identity string
		err = db.QueryRow("SELECT identity FROM campaign").Scan(&identity)
		if identity != l.identity {
			err = denied
		}
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	return l, nil
}

// A FULL-synchronous transaction commits before the sole HTTP RoundTrip.
// A reserved row after a crash blocks the whole campaign, including new IDs.
func (l *ledger) reserve(ctx context.Context, operation, body string) error {
	if err := privatePath(l.path, false); err != nil {
		return err
	}
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var identity string
	var revoked, failed, count, pending int
	if err = tx.QueryRowContext(ctx, "SELECT identity, revoked, failed FROM campaign").Scan(&identity, &revoked, &failed); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*), COALESCE(SUM(state='reserved'),0) FROM attempts").Scan(&count, &pending); err != nil {
		return err
	}
	if identity != l.identity || revoked != 0 || failed != 0 || count >= 8 || pending != 0 {
		return denied
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO attempts VALUES (?, ?, 'reserved')", operation, body); err != nil {
		return err
	}
	return tx.Commit()
}

func (l *ledger) finish(operation string, success bool) error {
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE attempts SET state='finished' WHERE operation=? AND state='reserved'", operation); err != nil {
		return err
	}
	if !success {
		if _, err = tx.Exec("UPDATE campaign SET failed=1"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type fakeUpstream struct {
	server        *http.Server
	listener      net.Listener
	key, endpoint string
	count         atomic.Int64
}

func ownFake(handler http.Handler) (*fakeUpstream, error) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	f := &fakeUpstream{listener: ln, key: token(), endpoint: "http://" + ln.Addr().String() + "/backend-api/codex/responses"}
	f.server = &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.RequestURI() != "/backend-api/codex/responses" || r.Header.Get("Authorization") != "Bearer "+f.key {
			http.Error(w, "denied", 403)
			return
		}
		f.count.Add(1)
		body, err := io.ReadAll(io.LimitReader(r.Body, wireRequestBytes+1))
		r.Body.Close()
		if err != nil || len(body) > wireRequestBytes {
			http.Error(w, "invalid fake request", 400)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		handler.ServeHTTP(w, r)
	})}
	go f.server.Serve(ln)
	return f, nil
}

func (f *fakeUpstream) close() { f.server.Close() }

type gateway struct {
	ledger    *ledger
	upstream  *fakeUpstream
	bound     binding
	key       string
	transport *http.Transport
	started   time.Time
	timeout   time.Duration
}

func newGateway(l *ledger, f *fakeUpstream, b binding, key string) (*gateway, error) {
	if f == nil || b.Endpoint != f.endpoint || b.Credential != digest([]byte(f.key)) || b.Key != digest([]byte(key)) || b.Account != "owned-offline-fixture" {
		return nil, denied
	}
	data, err := json.Marshal(b)
	if err != nil || digest(data) != l.identity {
		return nil, denied
	}
	// Caller-owned maps cannot add operations after durable binding validation.
	b = binding{}
	if err = json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	u, err := url.Parse(f.endpoint)
	if err != nil {
		return nil, err
	}
	address := u.Host
	tr := &http.Transport{DisableKeepAlives: true, ForceAttemptHTTP2: false, DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		if network != "tcp" || addr != address {
			return nil, denied
		}
		return (&net.Dialer{}).DialContext(ctx, "tcp4", address)
	}}
	return &gateway{ledger: l, upstream: f, bound: b, key: key, transport: tr, started: time.Now(), timeout: operationTimeout}, nil
}

func readObject(dec *json.Decoder) (map[string]any, error) {
	t, err := dec.Token()
	if err != nil || t != json.Delim('{') {
		return nil, denied
	}
	m := map[string]any{}
	for dec.More() {
		t, err = dec.Token()
		if err != nil {
			return nil, err
		}
		k, ok := t.(string)
		if !ok {
			return nil, denied
		}
		if _, exists := m[k]; exists {
			return nil, denied
		}
		var v any
		// Reject duplicate keys at every object depth, not just route controls.
		v, err = readValue(dec)
		if err != nil {
			return nil, err
		}
		m[k] = v
	}
	_, err = dec.Token()
	return m, err
}

func readValue(dec *json.Decoder) (any, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t {
	case json.Delim('{'):
		m := map[string]any{}
		for dec.More() {
			k, e := dec.Token()
			if e != nil {
				return nil, e
			}
			key, ok := k.(string)
			if !ok {
				return nil, denied
			}
			if _, exists := m[key]; exists {
				return nil, denied
			}
			v, e := readValue(dec)
			if e != nil {
				return nil, e
			}
			m[key] = v
		}
		_, err = dec.Token()
		return m, err
	case json.Delim('['):
		a := []any{}
		for dec.More() {
			v, e := readValue(dec)
			if e != nil {
				return nil, e
			}
			a = append(a, v)
		}
		_, err = dec.Token()
		return a, err
	}
	return t, nil
}

func validateBody(body []byte, effort string) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	m, err := readObject(dec)
	if err != nil {
		return err
	}
	if _, err = dec.Token(); err != io.EOF {
		return denied
	}
	allowed := map[string]bool{"model": true, "input": true, "instructions": true, "reasoning": true, "stream": true, "store": true, "max_output_tokens": true, "tools": true, "tool_choice": true, "parallel_tool_calls": true, "include": true}
	for k := range m {
		if !allowed[k] {
			return denied
		}
	}
	if m["model"] != "gpt-6.1-sol" || m["stream"] != true || m["store"] != false || m["input"] == nil {
		return denied
	}
	if !textOnlyInput(m["input"]) {
		return denied
	}
	switch m["input"].(type) {
	case string, []any:
	default:
		return denied
	}
	if choice, present := m["tool_choice"]; present {
		switch c := choice.(type) {
		case string:
			if c != "auto" && c != "none" && c != "required" {
				return denied
			}
		case map[string]any:
			if c["type"] != "function" {
				return denied
			}
		default:
			return denied
		}
	}
	reasoning, ok := m["reasoning"].(map[string]any)
	if !ok || len(reasoning) != 1 || reasoning["effort"] != effort {
		return denied
	}
	n, ok := m["max_output_tokens"].(json.Number)
	if !ok {
		return denied
	}
	limit, err := n.Int64()
	if err != nil || limit < 1 || limit > 512 {
		return denied
	}
	if raw, exists := m["tools"]; exists {
		tools, ok := raw.([]any)
		if !ok {
			return denied
		}
		for _, raw := range tools {
			tool, ok := raw.(map[string]any)
			if !ok || tool["type"] != "function" {
				return denied
			}
		}
	}
	return nil
}

func textOnlyInput(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for k, nested := range v {
			if k == "additional_tools" || k == "tools" || k == "file_id" || strings.HasSuffix(k, "_url") {
				return false
			}
			if k == "type" {
				switch nested {
				case "message", "input_text", "output_text", "function_call", "function_call_output", "reasoning", "summary_text", "reasoning_text":
				default:
					return false
				}
			}
			if !textOnlyInput(nested) {
				return false
			}
		}
	case []any:
		for _, nested := range v {
			if !textOnlyInput(nested) {
				return false
			}
		}
	}
	return true
}

func (g *gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reject := func() { http.Error(w, "research dispatch denied", http.StatusForbidden) }
	if r.Method != "POST" || r.URL.RequestURI() != "/v1/responses" || r.Header.Get("Content-Type") != "application/json" || len(r.Header.Values("Authorization")) != 1 || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+g.key)) != 1 {
		reject()
		return
	}
	for k := range r.Header {
		switch strings.ToLower(k) {
		case "authorization", "content-type", "x-research-operation", "user-agent", "accept", "accept-encoding", "content-length", "connection":
		default:
			reject()
			return
		}
	}
	op := r.Header.Get("X-Research-Operation")
	effort, ok := g.bound.Operations[op]
	if !ok || len(r.Header.Values("X-Research-Operation")) != 1 || time.Now().UnixMilli() >= g.bound.Expires || time.Since(g.started) >= 180*time.Second {
		reject()
		return
	}
	remaining := time.Until(time.UnixMilli(g.bound.Expires))
	if remaining > g.timeout {
		remaining = g.timeout
	}
	ctx, cancel := context.WithTimeout(r.Context(), remaining)
	defer cancel()
	controller := http.NewResponseController(w)
	if err := controller.SetReadDeadline(time.Now().Add(remaining)); err != nil {
		reject()
		return
	}
	if err := controller.SetWriteDeadline(time.Now().Add(remaining + time.Second)); err != nil {
		reject()
		return
	}
	// Closing the inbound body also bounds a slow request-body read.
	stop := context.AfterFunc(ctx, func() { r.Body.Close() })
	defer stop()
	body, err := io.ReadAll(io.LimitReader(r.Body, requestBytes+1))
	if err != nil || ctx.Err() != nil || len(body) > requestBytes || validateBody(body, effort) != nil {
		reject()
		return
	}
	body, err = service.ResearchOAuthTransform(body)
	if err != nil || len(body) > wireRequestBytes {
		reject()
		return
	}
	// Binding is immutable in memory and is checked against the durable row.
	if g.bound.Endpoint != g.upstream.endpoint || g.bound.Credential != digest([]byte(g.upstream.key)) {
		reject()
		return
	}
	req, err := http.NewRequestWithContext(ctx, "POST", g.bound.Endpoint, bytes.NewReader(body))
	if err != nil {
		reject()
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.upstream.key)
	if err = g.ledger.reserve(ctx, op, digest(body)); err != nil {
		reject()
		return
	}
	// RoundTrip has no redirect, rejected-field, account, or application retry.
	resp, err := g.transport.RoundTrip(req)
	if err != nil {
		g.ledger.finish(op, false)
		http.Error(w, "upstream failed", 502)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if err = g.ledger.finish(op, false); err != nil {
			reject()
			return
		}
		http.Error(w, "upstream rejected research request", resp.StatusCode)
		return
	}
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	var observed bytes.Buffer
	n, copyErr := io.Copy(io.MultiWriter(flushingWriter{w}, &observed), io.LimitReader(resp.Body, responseBytes))
	var extra [1]byte
	extraN, endErr := resp.Body.Read(extra[:])
	success := copyErr == nil && extraN == 0 && endErr == io.EOF && ctx.Err() == nil && n <= responseBytes && validTerminal(observed.Bytes())
	if err = g.ledger.finish(op, success); err != nil || !success {
		// An incomplete stream must not appear to have ended cleanly.
		panic(http.ErrAbortHandler)
	}
}

// Observation never turns usage into a financial cap. Invalid or missing
// terminal accounting stops further dispatch even if all HTTP bytes arrived.
func validTerminal(body []byte) bool {
	if !utf8.Valid(body) {
		return false
	}
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	if !strings.HasSuffix(text, "\n\n") {
		return false
	}
	completed := false
	sentinel := false
	for _, frame := range strings.Split(text, "\n\n") {
		data := []string{}
		for _, line := range strings.Split(frame, "\n") {
			if line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") {
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				return false
			}
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
		if len(data) == 0 {
			continue
		}
		payload := strings.Join(data, "\n")
		if payload == "[DONE]" {
			if !completed || sentinel {
				return false
			}
			sentinel = true
			continue
		}
		if completed || sentinel {
			return false
		}
		dec := json.NewDecoder(strings.NewReader(payload))
		dec.UseNumber()
		m, err := readObject(dec)
		if err != nil {
			return false
		}
		if _, err = dec.Token(); err != io.EOF {
			return false
		}
		typeName, ok := m["type"].(string)
		if !ok || !strings.HasPrefix(typeName, "response.") {
			return false
		}
		if typeName == "response.failed" || typeName == "response.incomplete" || typeName == "response.error" {
			return false
		}
		if typeName != "response.completed" {
			if _, present := m["usage"]; present {
				return false
			}
			continue
		}
		response, ok := m["response"].(map[string]any)
		if !ok || response["status"] != "completed" {
			return false
		}
		if model, present := response["model"]; present && model != "gpt-6.1-sol" {
			return false
		}
		usage, ok := response["usage"].(map[string]any)
		if !ok {
			return false
		}
		input, ok := usageCount(usage["input_tokens"])
		if !ok {
			return false
		}
		output, ok := usageCount(usage["output_tokens"])
		if !ok {
			return false
		}
		inputDetail, ok := usage["input_tokens_details"].(map[string]any)
		if !ok {
			return false
		}
		cached, ok := usageCount(inputDetail["cached_tokens"])
		if !ok || cached > input {
			return false
		}
		outputDetail, ok := usage["output_tokens_details"].(map[string]any)
		if !ok {
			return false
		}
		reasoning, ok := usageCount(outputDetail["reasoning_tokens"])
		if !ok || reasoning > output {
			return false
		}
		if total, present := usage["total_tokens"]; present {
			n, ok := usageCount(total)
			if !ok || n != input+output {
				return false
			}
		}
		if _, present := m["usage"]; present {
			return false
		}
		completed = true
	}
	return completed
}

func usageCount(value any) (int64, bool) {
	n, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	i, err := n.Int64()
	return i, err == nil && i >= 0 && i <= 9007199254740991
}

type flushingWriter struct{ http.ResponseWriter }

func (w flushingWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
	return n, err
}

// RunOffline owns both listeners, synthetic keys and a fresh disposable ledger.
// There is deliberately no upstream URL, credential input or live-mode flag.
func RunOffline() error {
	f, err := ownFake(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"offline\",\"status\":\"completed\",\"usage\":{\"input_tokens\":1,\"input_tokens_details\":{\"cached_tokens\":0},\"output_tokens\":1,\"output_tokens_details\":{\"reasoning_tokens\":0}}}}\n\n")
	}))
	if err != nil {
		return err
	}
	defer f.close()
	dir, err := os.MkdirTemp("", "sub2api-research-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	key := token()
	ops := map[string]string{}
	for i := 0; i < 8; i++ {
		effort := "medium"
		if i >= 4 {
			effort = "high"
		}
		ops[token()] = effort
	}
	b := binding{Source: SourceRevision, Model: "gpt-6.1-sol", Account: "owned-offline-fixture", Credential: digest([]byte(f.key)), Key: digest([]byte(key)), Endpoint: f.endpoint, Operations: ops, Expires: time.Now().Add(180 * time.Second).UnixMilli()}
	l, err := openLedger(filepath.Join(dir, "ledger.sqlite"), b, true)
	if err != nil {
		return err
	}
	defer l.db.Close()
	g, err := newGateway(l, f, b, key)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	s := &http.Server{Handler: g, ReadHeaderTimeout: time.Second, WriteTimeout: 21 * time.Second}
	go s.Serve(ln)
	defer s.Close()
	client := &http.Client{Timeout: 21 * time.Second, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}}
	for op, effort := range ops {
		body := fmt.Sprintf(`{"model":"gpt-6.1-sol","input":"offline","reasoning":{"effort":%q},"stream":true,"store":false,"max_output_tokens":512}`, effort)
		req, _ := http.NewRequest("POST", "http://"+ln.Addr().String()+"/v1/responses", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Research-Operation", op)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		_, err = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 {
			return denied
		}
	}
	if f.count.Load() != 8 {
		return denied
	}
	fmt.Println("offline research: 8 authorized operations, 8 fake dispatches, 0 live inference")
	return nil
}
