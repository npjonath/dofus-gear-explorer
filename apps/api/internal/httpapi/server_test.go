package httpapi

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"dofusdb/api/internal/gametest"
	"dofusdb/api/internal/store"
)

func newServer(t *testing.T, withData bool) *httptest.Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	srv := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)), "*", "")
	if withData {
		ds := gametest.Dataset()
		if err := st.Import(context.Background(), ds, "2026-01-01T00:00:00Z"); err != nil {
			t.Fatal(err)
		}
		srv.SetDataset(ds)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func getJSON(t *testing.T, url string, into any) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
	return resp.StatusCode
}

func postJSON(t *testing.T, url, body string, into any) int {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
	return resp.StatusCode
}

func TestDataRoutesWaitForAnImport(t *testing.T) {
	ts := newServer(t, false)
	var body apiError
	if status := getJSON(t, ts.URL+"/api/meta", &body); status != http.StatusServiceUnavailable || body.Error != "data_not_ready" {
		t.Errorf("status = %d, body = %+v", status, body)
	}
}

func TestItemsAreLocalizedAndFiltered(t *testing.T) {
	ts := newServer(t, true)
	var list struct {
		Total int
		Items []struct {
			ID        int
			Name      string
			TypeName  string
			Condition string
			Set       *struct{ Name string }
			Effects   []struct {
				Kind   string
				StatID *int
				Text   string
			}
		}
	}
	// Strength 10 at least 40, best first.
	status := getJSON(t, ts.URL+"/api/items?lang=en&stat=10:gte:40&sort=stat:10&order=desc", &list)
	if status != http.StatusOK || list.Total != 3 {
		t.Fatalf("status = %d, total = %d", status, list.Total)
	}
	if list.Items[0].ID != 50 || list.Items[1].ID != 40 || list.Items[2].ID != 11 {
		t.Errorf("order = %+v", list.Items)
	}
	hat := list.Items[1]
	if hat.TypeName != "Hat" || hat.Condition != "AP < 8" {
		t.Errorf("hat = %+v", hat)
	}
	if got := hat.Effects[1].Text; got != "30 à 50 Force" {
		t.Errorf("effect text = %q", got)
	}
	if list.Items[2].Set == nil || list.Items[2].Set.Name != "The Set" {
		t.Errorf("set = %+v", list.Items[2].Set)
	}

	var bad apiError
	if status := getJSON(t, ts.URL+"/api/items?stat=oops", &bad); status != http.StatusBadRequest {
		t.Errorf("malformed filter status = %d", status)
	}
}

func TestEvaluateAndSolve(t *testing.T) {
	ts := newServer(t, true)

	var evaluation struct {
		Stats map[string]int
		Valid bool
		Items map[string]struct{ Name string }
	}
	status := postJSON(t, ts.URL+"/api/builds/evaluate?lang=fr",
		`{"level":200,"items":{"ring1":10,"ring2":11,"amulet":30}}`, &evaluation)
	if status != http.StatusOK || !evaluation.Valid || evaluation.Stats["1"] != 9 {
		t.Errorf("status = %d, evaluation = %+v", status, evaluation)
	}
	if evaluation.Items["amulet"].Name != "Amulette du Set" {
		t.Errorf("items = %+v", evaluation.Items)
	}

	var solved struct {
		Solutions []struct {
			Feasible   bool
			Items      map[string]int
			Evaluation struct{ Stats map[string]int }
		}
		Unplaced *[]int
	}
	status = postJSON(t, ts.URL+"/api/solver/search?lang=fr",
		`{"character":{"level":200},"constraints":[{"statId":1,"op":"gte","value":9}],"maximize":[{"statId":10,"weight":1}]}`, &solved)
	if status != http.StatusOK || len(solved.Solutions) == 0 || !solved.Solutions[0].Feasible {
		t.Fatalf("status = %d, solutions = %+v", status, solved.Solutions)
	}
	if got := solved.Solutions[0].Evaluation.Stats["1"]; got < 9 {
		t.Errorf("AP of best solution = %d", got)
	}
	if solved.Unplaced == nil || len(*solved.Unplaced) != 0 {
		t.Errorf("unplaced = %v, want an empty list", solved.Unplaced)
	}

	// A required item is worn by every solution.
	status = postJSON(t, ts.URL+"/api/solver/search?lang=fr",
		`{"character":{"level":200},"maximize":[{"statId":10,"weight":1}],"required":[12]}`, &solved)
	if status != http.StatusOK || len(solved.Solutions) == 0 {
		t.Fatalf("status = %d, solutions = %+v", status, solved.Solutions)
	}
	for _, sol := range solved.Solutions {
		if sol.Items["ring1"] != 12 {
			t.Errorf("required ring missing: %v", sol.Items)
		}
	}

	var bad apiError
	if status := postJSON(t, ts.URL+"/api/solver/search", `{"character":{"level":200}}`, &bad); status != http.StatusBadRequest || bad.Error != "nothing_to_search" {
		t.Errorf("empty search: status = %d, body = %+v", status, bad)
	}
}

func TestClassesSpellsAndDamage(t *testing.T) {
	ts := newServer(t, true)

	var meta struct {
		Breeds []struct {
			ID   int
			Name string
		}
		Spells []struct {
			ID, BreedID int
			Name        string
			Damage      bool
		}
	}
	if status := getJSON(t, ts.URL+"/api/meta?lang=en", &meta); status != http.StatusOK {
		t.Fatalf("meta status = %d", status)
	}
	if len(meta.Breeds) != 1 || meta.Breeds[0].Name != "Cra" {
		t.Errorf("breeds = %+v", meta.Breeds)
	}
	// Class spells come first, then the common ones.
	if len(meta.Spells) != 2 || meta.Spells[0].BreedID != 9 || meta.Spells[1].BreedID != 0 || !meta.Spells[0].Damage {
		t.Errorf("spells = %+v", meta.Spells)
	}

	var damage struct {
		Weapon *struct{ ID int }
		Spells []struct {
			ID, Grade int
			Name      string
			Total     struct{ Min, Max, CritMin, CritMax int }
		}
	}
	// 60 Strength from the trophy: floor(20 x 1.6) = 32 to floor(30 x 1.6) = 48.
	status := postJSON(t, ts.URL+"/api/builds/damage?lang=fr",
		`{"level":200,"items":{"dofus1":50},"breedId":9}`, &damage)
	if status != http.StatusOK || damage.Weapon != nil || len(damage.Spells) != 2 {
		t.Fatalf("status = %d, damage = %+v", status, damage)
	}
	arrow := damage.Spells[0]
	if arrow.Name != "Flèche d'Essai" || arrow.Grade != 2 || arrow.Total.Min != 32 || arrow.Total.Max != 48 || arrow.Total.CritMax != 64 {
		t.Errorf("class spell = %+v", arrow)
	}
	// Without a class only the common spells are listed.
	status = postJSON(t, ts.URL+"/api/builds/damage?lang=fr", `{"level":200,"items":{}}`, &damage)
	if status != http.StatusOK || len(damage.Spells) != 1 || damage.Spells[0].Name != "Flamiche" {
		t.Errorf("no class: status = %d, spells = %+v", status, damage.Spells)
	}

	var bad apiError
	if status := postJSON(t, ts.URL+"/api/builds/damage", `{"level":200,"range":"far"}`, &bad); status != http.StatusBadRequest {
		t.Errorf("bad range: status = %d", status)
	}
	if status := postJSON(t, ts.URL+"/api/solver/search",
		`{"character":{"level":200},"damage":[{"kind":"weapon","id":40,"value":10}]}`, &bad); status != http.StatusBadRequest || bad.Error != "bad_damage_goal" {
		t.Errorf("bad damage goal: status = %d, body = %+v", status, bad)
	}
}

func TestSearchResultsArePaged(t *testing.T) {
	ts := newServer(t, true)
	type pageOf struct {
		ID        string
		Total     int
		Feasible  int
		Solutions []struct{ Items map[string]int }
	}
	var first pageOf
	body := `{"character":{"level":200},"maximize":[{"statId":10,"weight":1}],"maxResults":20}`
	if status := postJSON(t, ts.URL+"/api/solver/search?lang=fr&limit=1", body, &first); status != http.StatusOK {
		t.Fatalf("search status = %d", status)
	}
	if first.ID == "" || first.Total < 2 || len(first.Solutions) != 1 || first.Feasible != first.Total {
		t.Fatalf("first page: id %q, total %d, feasible %d, %d solutions", first.ID, first.Total, first.Feasible, len(first.Solutions))
	}

	// The next page continues where the first stopped.
	var next pageOf
	if status := getJSON(t, ts.URL+"/api/solver/results/"+first.ID+"?offset=1&limit=1", &next); status != http.StatusOK {
		t.Fatalf("results status = %d", status)
	}
	if next.Total != first.Total || len(next.Solutions) != 1 || reflect.DeepEqual(next.Solutions[0].Items, first.Solutions[0].Items) {
		t.Errorf("second page: total %d, %d solutions", next.Total, len(next.Solutions))
	}
	var beyond pageOf
	getJSON(t, ts.URL+"/api/solver/results/"+first.ID+"?offset=1000", &beyond)
	if len(beyond.Solutions) != 0 || beyond.Total != first.Total {
		t.Errorf("past the end: %d solutions", len(beyond.Solutions))
	}
	// Without a limit a page holds five stuffs.
	var plain pageOf
	postJSON(t, ts.URL+"/api/solver/search?lang=fr", body, &plain)
	if len(plain.Solutions) != min(5, plain.Total) {
		t.Errorf("default page: %d solutions of %d", len(plain.Solutions), plain.Total)
	}

	var gone apiError
	if status := getJSON(t, ts.URL+"/api/solver/results/unknown", &gone); status != http.StatusNotFound || gone.Error != "results_expired" {
		t.Errorf("unknown results: status %d, body %+v", status, gone)
	}
}

func TestItemsPageSizeIsCapped(t *testing.T) {
	ts := newServer(t, true)
	var list struct {
		Items []struct{ ID int }
		Total int
	}
	// The test dataset is small: what matters is that an oversized limit
	// still answers, with the items there are.
	if status := getJSON(t, ts.URL+"/api/items?limit=100000", &list); status != http.StatusOK || len(list.Items) != list.Total {
		t.Errorf("status %d, %d items of %d", status, len(list.Items), list.Total)
	}
	if status := getJSON(t, ts.URL+"/api/items?limit=2&offset=1", &list); status != http.StatusOK || len(list.Items) != 2 {
		t.Errorf("paged: status %d, %d items", status, len(list.Items))
	}
}

func TestAPIAnswersAreCompressed(t *testing.T) {
	ts := newServer(t, true)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/meta", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	// A transport that does not undo the compression shows the header.
	resp, err := (&http.Transport{DisableCompression: true}).RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("Content-Encoding = %q", resp.Header.Get("Content-Encoding"))
	}
	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var meta struct{ Version string }
	if err := json.NewDecoder(zr).Decode(&meta); err != nil || meta.Version == "" {
		t.Errorf("meta = %+v, err %v", meta, err)
	}
}
