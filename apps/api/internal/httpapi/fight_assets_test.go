package httpapi

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
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
	if err := os.WriteFile(filepath.Join(dir, "..", "complete.json"), []byte(`{"format":1,"id":"pack-test"}`), 0644); err != nil {
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

func TestFightAssetsReusePublishedPacksAfterRestart(t *testing.T) {
	dir := t.TempDir()
	primary := filepath.Join(dir, "fight-assets")
	fallback := filepath.Join(t.TempDir(), "fight-assets")
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	publish := func(root, id string) {
		pack := filepath.Join(root, id)
		write(filepath.Join(pack, "maps", "192419850.json"), `{"id":192419850,"width":3,"height":3,"blue":[0,1,2,3],"red":[4,5,6,7],"imageWidth":30,"imageHeight":30,"halfCellWidth":5,"halfCellHeight":3}`)
		write(filepath.Join(pack, "hud", "sample.json"), id)
		write(filepath.Join(pack, "complete.json"), `{"format":1,"id":"`+id+`"}`)
		write(filepath.Join(root, "current"), id)
	}
	open := func(fallbackDir string) http.Handler {
		s := new(Server)
		s.SetUpdater(updater.New(nil, nil, updater.Options{DataDir: dir, FightAssetsFallbackDir: fallbackDir}, nil, nil))
		return s.Handler()
	}
	check := func(handler http.Handler, expected string) {
		t.Helper()
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/api/fight/assets", nil))
		var data struct {
			Pack string `json:"pack"`
			Maps []int  `json:"maps"`
		}
		if err := json.Unmarshal(result.Body.Bytes(), &data); err != nil || result.Code != http.StatusOK || data.Pack != expected {
			t.Fatalf("asset discovery: %d %s (%v), want %q", result.Code, result.Body.String(), err, expected)
		}
		if expected == "" {
			if len(data.Maps) != 0 {
				t.Fatalf("unexpected maps with no published pack: %v", data.Maps)
			}
			return
		}
		if !slices.Equal(data.Maps, []int{192419850}) {
			t.Fatalf("maps for %q: %v", expected, data.Maps)
		}
		result = httptest.NewRecorder()
		handler.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/fight-assets/hud/sample.json", nil))
		if result.Code != http.StatusOK || result.Body.String() != expected {
			t.Fatalf("read pack %q: %d %s", expected, result.Code, result.Body.String())
		}
	}
	publish(fallback, "pack-other-profile")
	write(filepath.Join(primary, "current"), "pack-incomplete")
	write(filepath.Join(primary, "pack-incomplete", "hud", "sample.json"), "incomplete")
	check(open(""), "")
	server := open(fallback)
	check(server, "pack-other-profile")
	check(open(fallback), "pack-other-profile")
	publish(primary, "pack-this-profile")
	check(server, "pack-this-profile")
	check(open(fallback), "pack-this-profile")
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
