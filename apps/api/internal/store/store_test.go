package store

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"dofusdb/api/internal/game"
	"dofusdb/api/internal/gametest"
)

func openImported(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.Import(context.Background(), gametest.Dataset(), "2026-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestImportRoundTrip(t *testing.T) {
	st := openImported(t)
	got, err := st.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := gametest.Dataset()
	if got.Version != want.Version || !reflect.DeepEqual(got.Languages, want.Languages) {
		t.Errorf("version/languages = %q %v", got.Version, got.Languages)
	}
	if !reflect.DeepEqual(got.Items, want.Items) {
		t.Errorf("items differ after a round trip:\n got %+v\nwant %+v", got.Items, want.Items)
	}
	if !reflect.DeepEqual(got.Sets[0].Bonuses, want.Sets[0].Bonuses) {
		t.Errorf("set bonuses = %+v", got.Sets[0].Bonuses)
	}
	if len(got.Stats) != len(want.Stats) || len(got.Effects) != len(want.Effects) || len(got.Types) != len(want.Types) {
		t.Errorf("stats/effects/types counts differ")
	}
	if !reflect.DeepEqual(got.Breeds, want.Breeds) {
		t.Errorf("breeds = %+v", got.Breeds)
	}
	if !reflect.DeepEqual(got.Spells, want.Spells) {
		t.Errorf("spells differ after a round trip:\n got %+v\nwant %+v", got.Spells, want.Spells)
	}
}

func TestImportReplacesPreviousRelease(t *testing.T) {
	st := openImported(t)
	next := gametest.Dataset()
	next.Version = "test-2"
	next.Items = next.Items[:2]
	next.Sets = nil
	for i := range next.Items {
		next.Items[i].SetID = 0
	}
	if err := st.Import(context.Background(), next, "2026-02-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	got, err := st.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "test-2" || len(got.Items) != 2 || len(got.Sets) != 0 {
		t.Errorf("after re-import: version %q, %d items, %d sets", got.Version, len(got.Items), len(got.Sets))
	}
}

func TestQueryItems(t *testing.T) {
	st := openImported(t)
	ctx := context.Background()
	query := func(q ItemQuery) []int {
		t.Helper()
		q.Lang = "fr"
		ids, total, err := st.QueryItems(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		if total != len(ids) {
			t.Errorf("total = %d, ids = %v", total, ids)
		}
		return ids
	}

	if got := query(ItemQuery{Search: "eloigne"}); !reflect.DeepEqual(got, []int{12}) {
		t.Errorf("accent-insensitive search = %v", got)
	}
	if got := query(ItemQuery{Slots: []string{string(game.SlotRing)}, Sort: "level", Desc: true}); !reflect.DeepEqual(got, []int{12, 11, 10}) {
		t.Errorf("rings by level = %v", got)
	}
	strength := StatFilter{StatID: game.StatStrength, Op: "gte", Value: 40}
	if got := query(ItemQuery{Stats: []StatFilter{strength}, Sort: "stat", SortStatID: game.StatStrength, Desc: true}); !reflect.DeepEqual(got, []int{50, 40, 11}) {
		t.Errorf("strength >= 40 = %v", got)
	}
	noStrength := StatFilter{StatID: game.StatStrength, Op: "lte", Value: 0}
	if got := query(ItemQuery{Stats: []StatFilter{noStrength}, Sort: "name"}); !reflect.DeepEqual(got, []int{10, 12, 51}) {
		t.Errorf("strength <= 0 = %v", got)
	}
	if got := query(ItemQuery{Slots: []string{string(game.SlotRing)}, ExcludeTypeIDs: []int{9}}); len(got) != 0 {
		t.Errorf("excluding the ring type should leave no ring, got %v", got)
	}
	if got := query(ItemQuery{ExcludeTypeIDs: []int{151}, Sort: "name"}); !reflect.DeepEqual(got, []int{30, 10, 12, 11, 40}) {
		t.Errorf("without trophies = %v", got)
	}
	if got := query(ItemQuery{LevelMin: 90, LevelMax: 100, Sort: "name"}); !reflect.DeepEqual(got, []int{40, 50, 51}) {
		t.Errorf("level 90-100 = %v", got)
	}
}
