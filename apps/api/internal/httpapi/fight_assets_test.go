package httpapi

import (
	"bytes"
	"compress/gzip"
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
