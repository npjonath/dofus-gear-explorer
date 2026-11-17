package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"dofusdb/api/internal/store"
)

const devOrigin = "http://localhost:5173"

func send(t *testing.T, ts *httptest.Server, method, path, host, origin string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if host != "" {
		req.Host = host
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "text/plain")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func TestGuard(t *testing.T) {
	ts := newServer(t, false)
	self := strings.TrimPrefix(ts.URL, "http://")

	cases := []struct {
		name, method, path, host, origin string
		status                           int
		allowOrigin                      string
	}{
		{"desktop or curl, no origin", http.MethodGet, "/api/health", "", "", http.StatusOK, ""},
		{"allowed origin", http.MethodGet, "/api/health", "", devOrigin, http.StatusOK, devOrigin},
		{"allowed origin preflight", http.MethodOptions, "/api/data/update", "", devOrigin, http.StatusNoContent, devOrigin},
		{"same origin", http.MethodGet, "/api/health", "", ts.URL, http.StatusOK, ""},
		{"localhost host", http.MethodGet, "/api/health", "localhost:" + strings.Split(self, ":")[1], "", http.StatusOK, ""},
		{"foreign page posts", http.MethodPost, "/api/data/update", "", "https://evil.example", http.StatusForbidden, ""},
		{"foreign page reads", http.MethodGet, "/api/meta", "", "https://evil.example", http.StatusForbidden, ""},
		{"foreign preflight", http.MethodOptions, "/api/data/update", "", "https://evil.example", http.StatusForbidden, ""},
		{"sandboxed frame", http.MethodPost, "/api/data/check", "", "null", http.StatusForbidden, ""},
		{"rebound domain", http.MethodPost, "/api/data/update", "evil.example:80", "http://evil.example:80", http.StatusForbidden, ""},
		{"rebound domain without origin", http.MethodGet, "/api/health", "evil.example", "", http.StatusForbidden, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := send(t, ts, c.method, c.path, c.host, c.origin)
			if resp.StatusCode != c.status {
				t.Fatalf("status %d, want %d", resp.StatusCode, c.status)
			}
			if got := resp.Header.Get("Access-Control-Allow-Origin"); got != c.allowOrigin {
				t.Fatalf("Access-Control-Allow-Origin %q, want %q", got, c.allowOrigin)
			}
		})
	}
}

func TestGuardPublic(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	srv := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)), Access{AllowedOrigins: []string{"*"}}, "")
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp := send(t, ts, http.MethodGet, "/api/health", "api.example.org", "https://anywhere.example")
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("status %d, allow origin %q", resp.StatusCode, resp.Header.Get("Access-Control-Allow-Origin"))
	}
}

func TestIsLoopbackHost(t *testing.T) {
	for host, want := range map[string]bool{
		"127.0.0.1:8787": true, "127.0.0.1:0": true, "localhost": true, "LOCALHOST:5173": true,
		"app.localhost": true, "[::1]:8787": true, "::1": true, "127.0.0.2": true,
		":8787": false, "0.0.0.0:8787": false, "192.168.1.10:8787": false, "evil.example": false,
		"localhost.evil.example": false,
	} {
		if got := IsLoopbackHost(host); got != want {
			t.Errorf("IsLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}
