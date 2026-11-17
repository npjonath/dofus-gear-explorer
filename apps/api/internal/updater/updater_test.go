package updater

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"dofusdb/api/internal/gametest"
	"dofusdb/api/internal/store"
)

func TestLangsMissing(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	// The test dataset holds French and English.
	if err := st.Import(context.Background(), gametest.Dataset(), "2026-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	cases := []struct {
		langs   []string
		version string
		want    bool
	}{
		{[]string{"fr", "en"}, "3.7.14.18", false},
		{[]string{"en"}, "3.7.14.18", false},
		{[]string{"fr", "en", "es", "pt"}, "3.7.14.18", true},
		// Local imports have no release to take the translations from.
		{[]string{"fr", "en", "es", "pt"}, LocalVersionPrefix + "20260101-000000", false},
	}
	for _, c := range cases {
		u := New(st, nil, Options{Langs: c.langs}, log, nil)
		if got := u.langsMissing(context.Background(), c.version); got != c.want {
			t.Errorf("langs %v, version %s: langsMissing = %v, want %v", c.langs, c.version, got, c.want)
		}
	}
}
