package httpapi

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"dofusdb/api/internal/updater"
)

func TestImportedAtlasServesBothEncodings(t *testing.T) {
	data := t.TempDir()
	s := new(Server)
	s.SetUpdater(updater.New(nil, nil, updater.Options{DataDir: data}, nil, nil))
	root := filepath.Join(data, "fight-assets")
	dir := filepath.Join(root, "pack-test", "actors")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "current"), []byte("pack-test"), 0644); err != nil {
		t.Fatal(err)
	}
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	_, _ = zw.Write([]byte(`{"textures":[]}`))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skin.json.gz"), compressed.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	for _, accept := range []string{"", "gzip"} {
		req := httptest.NewRequest("GET", "/fight-assets/actors/skin.json", nil)
		req.Header.Set("Accept-Encoding", accept)
		out := httptest.NewRecorder()
		s.Handler().ServeHTTP(out, req)
		want := []byte(`{"textures":[]}`)
		if accept == "gzip" {
			want = compressed.Bytes()
		}
		if out.Code != 200 || out.Header().Get("Content-Encoding") != accept || !bytes.Equal(out.Body.Bytes(), want) {
			t.Fatalf("encoding %q: %d %q", accept, out.Code, out.Body.String())
		}
	}
}

func TestDesktopCanUseImportedFontsAndMasks(t *testing.T) {
	called := false
	s := &Server{access: Access{LocalOnly: true}}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	for _, path := range []string{"/fight-assets/hud/end-idle.png", "/api/fight"} {
		called = false
		q := httptest.NewRequest("GET", "http://127.0.0.1"+path, nil)
		q.Header.Set("Origin", "null")
		w := httptest.NewRecorder()
		s.guard(next).ServeHTTP(w, q)
		if path == "/api/fight" {
			if called || w.Code != 403 {
				t.Fatal("unexpected private API access")
			}
		} else if !called || w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatal("opaque origin cannot load HUD")
		}
	}
}
