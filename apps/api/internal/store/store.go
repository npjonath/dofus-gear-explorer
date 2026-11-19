// Package store persists the imported game data in SQLite.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"dofusdb/api/internal/game"
)

// schemaVersion changes whenever the tables below change shape. A database
// written by another schema version is rebuilt and its data re-imported.
const schemaVersion = "12"

// Meta keys.
const (
	MetaSchemaVersion = "schema_version"
	MetaDataVersion   = "data_version"
	MetaImportedAt    = "imported_at"
	MetaLatestVersion = "latest_version"
	MetaLastCheckedAt = "last_checked_at"
	MetaImagesVersion = "images_version"
	// MetaImagesSize is the pixel size of the icons on disk.
	MetaImagesSize = "images_size"
)

const schema = `
CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS languages (
	code TEXT PRIMARY KEY
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS stat_categories (
	id         INTEGER PRIMARY KEY,
	sort_order INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS stats (
	id          INTEGER PRIMARY KEY,
	keyword     TEXT NOT NULL,
	category_id INTEGER NOT NULL,
	sort_order  INTEGER NOT NULL,
	unit        TEXT NOT NULL,
	weight      REAL NOT NULL
);

CREATE TABLE IF NOT EXISTS effects (
	id      INTEGER PRIMARY KEY,
	kind    TEXT NOT NULL CHECK (kind IN ('stat', 'weapon')),
	stat_id INTEGER NOT NULL,
	sign    INTEGER NOT NULL,
	element INTEGER NOT NULL
);

-- Character classes, and the spells a character can cast with the damage
-- lines of each grade (JSON).
CREATE TABLE IF NOT EXISTS breeds (
	id    INTEGER PRIMARY KEY,
	ord   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS spells (
	id         INTEGER PRIMARY KEY,
	icon_id    INTEGER NOT NULL,
	breed_id   INTEGER NOT NULL,
	ord        INTEGER NOT NULL,
	variant_of INTEGER NOT NULL,
	starts_for INTEGER NOT NULL,
	levels     TEXT NOT NULL
);

-- Transcendence runes: one fixed bonus each, and the weight it counts for.
CREATE TABLE IF NOT EXISTS runes (
	id        INTEGER PRIMARY KEY,
	level     INTEGER NOT NULL,
	effect_id INTEGER NOT NULL,
	stat_id   INTEGER NOT NULL,
	value     INTEGER NOT NULL,
	density   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS item_types (
	id            INTEGER PRIMARY KEY,
	super_type_id INTEGER NOT NULL,
 raw_zone TEXT NOT NULL,
	slot          TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sets (
	id    INTEGER PRIMARY KEY,
	level INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS items (
	id                INTEGER PRIMARY KEY,
	type_id           INTEGER NOT NULL REFERENCES item_types(id),
	slot              TEXT NOT NULL,
	level             INTEGER NOT NULL,
	icon_id           INTEGER NOT NULL,
	set_id            INTEGER NOT NULL,
	criteria          TEXT NOT NULL,
	pods              INTEGER NOT NULL,
	is_weapon         INTEGER NOT NULL CHECK (is_weapon IN (0, 1)),
	ap_cost           INTEGER NOT NULL,
	range_min         INTEGER NOT NULL,
	range_max         INTEGER NOT NULL,
	crit_chance       INTEGER NOT NULL,
	crit_bonus        INTEGER NOT NULL,
	max_cast_per_turn INTEGER NOT NULL,
	passive_spell     INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS items_slot_level ON items(slot, level);
CREATE INDEX IF NOT EXISTS items_type ON items(type_id);
CREATE INDEX IF NOT EXISTS items_set ON items(set_id);

CREATE TABLE IF NOT EXISTS item_effects (
	item_id   INTEGER NOT NULL REFERENCES items(id),
	position  INTEGER NOT NULL,
	effect_id INTEGER NOT NULL,
	kind      TEXT NOT NULL,
	stat_id   INTEGER NOT NULL,
	min_value INTEGER NOT NULL,
	max_value INTEGER NOT NULL,
	PRIMARY KEY (item_id, position)
) WITHOUT ROWID;

-- One row per (item, stat): the summed range, used to filter and sort.
CREATE TABLE IF NOT EXISTS item_stats (
	item_id   INTEGER NOT NULL REFERENCES items(id),
	stat_id   INTEGER NOT NULL,
	min_value INTEGER NOT NULL,
	max_value INTEGER NOT NULL,
	PRIMARY KEY (item_id, stat_id)
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS item_stats_stat ON item_stats(stat_id, max_value);

CREATE TABLE IF NOT EXISTS set_items (
	set_id  INTEGER NOT NULL REFERENCES sets(id),
	item_id INTEGER NOT NULL REFERENCES items(id),
	PRIMARY KEY (set_id, item_id)
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS set_bonuses (
	set_id     INTEGER NOT NULL REFERENCES sets(id),
	item_count INTEGER NOT NULL,
	position   INTEGER NOT NULL,
	effect_id  INTEGER NOT NULL,
	stat_id    INTEGER NOT NULL,
	value      INTEGER NOT NULL,
	PRIMARY KEY (set_id, item_count, position)
) WITHOUT ROWID;

-- Every translated string. search is the accent-folded text, set for names.
CREATE TABLE IF NOT EXISTS texts (
	kind   TEXT NOT NULL,
	id     INTEGER NOT NULL,
	lang   TEXT NOT NULL,
	text   TEXT NOT NULL,
	search TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (kind, id, lang)
) WITHOUT ROWID;
`

var dataTables = []string{
	"texts", "set_bonuses", "set_items", "item_stats", "item_effects",
	"items", "sets", "item_types", "effects", "stats", "stat_categories", "languages", "runes",
	"breeds", "spells",
}

// Text kinds stored in the texts table.
const (
	textItem     = "item"
	textItemDesc = "item_desc"
	textType     = "type"
	textSet      = "set"
	textStat     = "stat"
	textCategory = "stat_category"
	textEffect   = "effect"
	textRune     = "rune"
	textBreed    = "breed"
	textSpell    = "spell"
	// A passive belongs to its item, so both kinds are keyed by item id.
	textPassiveName = "passive_name"
	textPassiveDesc = "passive_desc"
)

type Store struct {
	db   *sql.DB
	path string
}

// Open opens (and creates if needed) the database at path.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(0)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	current, err := s.Meta(MetaSchemaVersion)
	if err != nil {
		return err
	}
	if current == schemaVersion {
		return nil
	}
	if current != "" {
		// Game data is disposable: drop it and let the updater import it again.
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		for _, table := range dataTables {
			if _, err := tx.Exec("DROP TABLE IF EXISTS " + table); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(schema); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM meta WHERE key IN (?, ?)", MetaDataVersion, MetaImportedAt); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return s.SetMeta(MetaSchemaVersion, schemaVersion)
}

// Meta returns a meta value, or "" when it is not set.
func (s *Store) Meta(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

// Languages returns the language codes of the imported game data.
func (s *Store) Languages(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT code FROM languages ORDER BY code")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, rows.Err()
}

func (s *Store) SetMeta(key, value string) error {
	_, err := s.db.Exec(
		"INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	return err
}

// Import replaces all game data with ds in a single transaction, so readers
// see either the previous release or the new one, never a mix.
func (s *Store) Import(ctx context.Context, ds *game.Dataset, importedAt string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, table := range dataTables {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}

	prepare := func(query string) (*sql.Stmt, error) { return tx.PrepareContext(ctx, query) }

	insText, err := prepare("INSERT INTO texts(kind, id, lang, text, search) VALUES(?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer insText.Close()
	putTexts := func(kind string, id int, texts game.Texts, searchable bool) error {
		for lang, text := range texts {
			search := ""
			if searchable {
				search = game.Fold(text)
			}
			if _, err := insText.ExecContext(ctx, kind, id, lang, text, search); err != nil {
				return err
			}
		}
		return nil
	}

	for _, lang := range ds.Languages {
		if _, err := tx.ExecContext(ctx, "INSERT INTO languages(code) VALUES(?)", lang); err != nil {
			return err
		}
	}
	for _, c := range ds.Categories {
		if _, err := tx.ExecContext(ctx, "INSERT INTO stat_categories(id, sort_order) VALUES(?, ?)", c.ID, c.Order); err != nil {
			return err
		}
		if err := putTexts(textCategory, c.ID, c.Name, false); err != nil {
			return err
		}
	}
	for _, st := range ds.Stats {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO stats(id, keyword, category_id, sort_order, unit, weight) VALUES(?, ?, ?, ?, ?, ?)",
			st.ID, st.Key, st.CategoryID, st.Order, st.Unit, st.Weight); err != nil {
			return err
		}
		if err := putTexts(textStat, st.ID, st.Label, false); err != nil {
			return err
		}
	}
	for _, e := range ds.Effects {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO effects(id, kind, stat_id, sign, element) VALUES(?, ?, ?, ?, ?)",
			e.ID, string(e.Kind), e.StatID, e.Sign, e.Element); err != nil {
			return err
		}
		if err := putTexts(textEffect, e.ID, e.Template, false); err != nil {
			return err
		}
	}
	for _, r := range ds.Runes {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO runes(id, level, effect_id, stat_id, value, density) VALUES(?, ?, ?, ?, ?, ?)",
			r.ID, r.Level, r.EffectID, r.StatID, r.Value, r.Density); err != nil {
			return err
		}
		if err := putTexts(textRune, r.ID, r.Name, false); err != nil {
			return err
		}
	}
	for _, b := range ds.Breeds {
		if _, err := tx.ExecContext(ctx, "INSERT INTO breeds(id, ord) VALUES(?, ?)", b.ID, b.Order); err != nil {
			return err
		}
		if err := putTexts(textBreed, b.ID, b.Name, false); err != nil {
			return err
		}
	}
	for _, sp := range ds.Spells {
		levels, err := json.Marshal(sp.Levels)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO spells(id, icon_id, breed_id, ord, variant_of, starts_for, levels) VALUES(?, ?, ?, ?, ?, ?, ?)",
			sp.ID, sp.IconID, sp.BreedID, sp.Order, sp.VariantOf, sp.StartsFor, string(levels)); err != nil {
			return err
		}
		if err := putTexts("spell_desc", sp.ID, sp.Description, false); err != nil {
			return err
		}
		if err := putTexts(textSpell, sp.ID, sp.Name, false); err != nil {
			return err
		}
	}
	for _, t := range ds.Types {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO item_types(id, super_type_id, slot,raw_zone) VALUES(?, ?, ?,?)",
			t.ID, t.SuperTypeID, string(t.Slot), t.RawZone); err != nil {
			return err
		}
		if err := putTexts(textType, t.ID, t.Name, false); err != nil {
			return err
		}
	}

	insItem, err := prepare(`INSERT INTO items(id, type_id, slot, level, icon_id, set_id, criteria, pods,
		is_weapon, ap_cost, range_min, range_max, crit_chance, crit_bonus, max_cast_per_turn, passive_spell)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insItem.Close()
	insEffect, err := prepare("INSERT INTO item_effects(item_id, position, effect_id, kind, stat_id, min_value, max_value) VALUES(?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer insEffect.Close()
	insStat, err := prepare(`INSERT INTO item_stats(item_id, stat_id, min_value, max_value) VALUES(?, ?, ?, ?)
		ON CONFLICT(item_id, stat_id) DO UPDATE SET
			min_value = min_value + excluded.min_value,
			max_value = max_value + excluded.max_value`)
	if err != nil {
		return err
	}
	defer insStat.Close()

	for i := range ds.Items {
		it := &ds.Items[i]
		w := game.WeaponInfo{}
		isWeapon := 0
		if it.Weapon != nil {
			w, isWeapon = *it.Weapon, 1
		}
		if _, err := insItem.ExecContext(ctx, it.ID, it.TypeID, string(it.Slot), it.Level, it.IconID, it.SetID,
			it.Criteria, it.Pods, isWeapon, w.APCost, w.RangeMin, w.RangeMax, w.CritChance, w.CritBonus, w.MaxCastPerTurn, passiveSpell(it)); err != nil {
			return fmt.Errorf("insert item %d: %w", it.ID, err)
		}
		for pos, e := range it.Effects {
			if _, err := insEffect.ExecContext(ctx, it.ID, pos, e.EffectID, string(e.Kind), e.StatID, e.Min, e.Max); err != nil {
				return fmt.Errorf("insert effect of item %d: %w", it.ID, err)
			}
			if e.Kind == game.EffectStat {
				if _, err := insStat.ExecContext(ctx, it.ID, e.StatID, e.Min, e.Max); err != nil {
					return err
				}
			}
		}
		if err := putTexts(textItem, it.ID, it.Name, true); err != nil {
			return err
		}
		if err := putTexts(textItemDesc, it.ID, it.Description, false); err != nil {
			return err
		}
		if it.Passive != nil {
			if err := putTexts(textPassiveName, it.ID, it.Passive.Name, true); err != nil {
				return err
			}
			if err := putTexts(textPassiveDesc, it.ID, it.Passive.Description, false); err != nil {
				return err
			}
		}
	}

	for i := range ds.Sets {
		set := &ds.Sets[i]
		if _, err := tx.ExecContext(ctx, "INSERT INTO sets(id, level) VALUES(?, ?)", set.ID, set.Level); err != nil {
			return err
		}
		for _, itemID := range set.ItemIDs {
			if _, err := tx.ExecContext(ctx, "INSERT INTO set_items(set_id, item_id) VALUES(?, ?)", set.ID, itemID); err != nil {
				return err
			}
		}
		for count, bonuses := range set.Bonuses {
			for pos, b := range bonuses {
				if _, err := tx.ExecContext(ctx,
					"INSERT INTO set_bonuses(set_id, item_count, position, effect_id, stat_id, value) VALUES(?, ?, ?, ?, ?, ?)",
					set.ID, count, pos, b.EffectID, b.StatID, b.Value); err != nil {
					return err
				}
			}
		}
		if err := putTexts(textSet, set.ID, set.Name, true); err != nil {
			return err
		}
	}

	setMeta := func(key, value string) error {
		_, err := tx.ExecContext(ctx,
			"INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
			key, value)
		return err
	}
	if err := setMeta(MetaDataVersion, ds.Version); err != nil {
		return err
	}
	if err := setMeta(MetaImportedAt, importedAt); err != nil {
		return err
	}
	return tx.Commit()
}

// Load reads the whole dataset back. It returns nil when nothing is imported.
func (s *Store) Load(ctx context.Context) (*game.Dataset, error) {
	version, err := s.Meta(MetaDataVersion)
	if err != nil || version == "" {
		return nil, err
	}
	ds := &game.Dataset{Version: version}

	type textKey struct {
		kind string
		id   int
	}
	texts := map[textKey]game.Texts{}
	rows, err := s.db.QueryContext(ctx, "SELECT kind, id, lang, text FROM texts")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var k textKey
		var lang, text string
		if err := rows.Scan(&k.kind, &k.id, &lang, &text); err != nil {
			rows.Close()
			return nil, err
		}
		if texts[k] == nil {
			texts[k] = game.Texts{}
		}
		texts[k][lang] = text
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	textsOf := func(kind string, id int) game.Texts {
		if t := texts[textKey{kind, id}]; t != nil {
			return t
		}
		return game.Texts{}
	}

	scan := func(query string, each func(*sql.Rows) error) error {
		rows, err := s.db.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			if err := each(rows); err != nil {
				return err
			}
		}
		return rows.Err()
	}

	if err := scan("SELECT code FROM languages ORDER BY code", func(r *sql.Rows) error {
		var code string
		if err := r.Scan(&code); err != nil {
			return err
		}
		ds.Languages = append(ds.Languages, code)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := scan("SELECT id, sort_order FROM stat_categories ORDER BY sort_order", func(r *sql.Rows) error {
		var c game.StatCategory
		if err := r.Scan(&c.ID, &c.Order); err != nil {
			return err
		}
		c.Name = textsOf(textCategory, c.ID)
		ds.Categories = append(ds.Categories, c)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := scan("SELECT id, keyword, category_id, sort_order, unit, weight FROM stats ORDER BY id", func(r *sql.Rows) error {
		var st game.Stat
		if err := r.Scan(&st.ID, &st.Key, &st.CategoryID, &st.Order, &st.Unit, &st.Weight); err != nil {
			return err
		}
		st.Label = textsOf(textStat, st.ID)
		ds.Stats = append(ds.Stats, st)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := scan("SELECT id, kind, stat_id, sign, element FROM effects ORDER BY id", func(r *sql.Rows) error {
		var e game.EffectDef
		var kind string
		if err := r.Scan(&e.ID, &kind, &e.StatID, &e.Sign, &e.Element); err != nil {
			return err
		}
		e.Kind = game.EffectKind(kind)
		e.Template = textsOf(textEffect, e.ID)
		ds.Effects = append(ds.Effects, e)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := scan("SELECT id, super_type_id, slot,raw_zone FROM item_types ORDER BY id", func(r *sql.Rows) error {
		var t game.ItemType
		var slot string
		if err := r.Scan(&t.ID, &t.SuperTypeID, &slot, &t.RawZone); err != nil {
			return err
		}
		t.Slot = game.Slot(slot)
		t.Name = textsOf(textType, t.ID)
		ds.Types = append(ds.Types, t)
		return nil
	}); err != nil {
		return nil, err
	}

	if err := scan("SELECT id, level, effect_id, stat_id, value, density FROM runes ORDER BY id", func(r *sql.Rows) error {
		var rn game.Rune
		if err := r.Scan(&rn.ID, &rn.Level, &rn.EffectID, &rn.StatID, &rn.Value, &rn.Density); err != nil {
			return err
		}
		rn.Name = textsOf(textRune, rn.ID)
		ds.Runes = append(ds.Runes, rn)
		return nil
	}); err != nil {
		return nil, err
	}

	if err := scan("SELECT id, ord FROM breeds ORDER BY ord, id", func(r *sql.Rows) error {
		var b game.Breed
		if err := r.Scan(&b.ID, &b.Order); err != nil {
			return err
		}
		b.Name = textsOf(textBreed, b.ID)
		ds.Breeds = append(ds.Breeds, b)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := scan("SELECT id, icon_id, breed_id, ord, variant_of, starts_for, levels FROM spells ORDER BY id", func(r *sql.Rows) error {
		var sp game.Spell
		var levels string
		if err := r.Scan(&sp.ID, &sp.IconID, &sp.BreedID, &sp.Order, &sp.VariantOf, &sp.StartsFor, &levels); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(levels), &sp.Levels); err != nil {
			return fmt.Errorf("spell %d: %w", sp.ID, err)
		}
		sp.Name = textsOf(textSpell, sp.ID)
		sp.Description = texts[textKey{"spell_desc", sp.ID}]
		ds.Spells = append(ds.Spells, sp)
		return nil
	}); err != nil {
		return nil, err
	}

	itemIndex := map[int]int{}
	if err := scan(`SELECT id, type_id, slot, level, icon_id, set_id, criteria, pods, is_weapon,
		ap_cost, range_min, range_max, crit_chance, crit_bonus, max_cast_per_turn, passive_spell FROM items ORDER BY id`,
		func(r *sql.Rows) error {
			var it game.Item
			var slot string
			var isWeapon int
			var w game.WeaponInfo
			passive := 0
			if err := r.Scan(&it.ID, &it.TypeID, &slot, &it.Level, &it.IconID, &it.SetID, &it.Criteria, &it.Pods,
				&isWeapon, &w.APCost, &w.RangeMin, &w.RangeMax, &w.CritChance, &w.CritBonus, &w.MaxCastPerTurn, &passive); err != nil {
				return err
			}
			it.Slot = game.Slot(slot)
			if isWeapon == 1 {
				it.Weapon = &w
			}
			it.Name = textsOf(textItem, it.ID)
			it.Description = textsOf(textItemDesc, it.ID)
			if name := texts[textKey{textPassiveName, it.ID}]; name != nil {
				it.Passive = &game.Passive{Name: name, Description: textsOf(textPassiveDesc, it.ID), SpellID: passive}
			}
			itemIndex[it.ID] = len(ds.Items)
			ds.Items = append(ds.Items, it)
			return nil
		}); err != nil {
		return nil, err
	}
	if err := scan("SELECT item_id, effect_id, kind, stat_id, min_value, max_value FROM item_effects ORDER BY item_id, position",
		func(r *sql.Rows) error {
			var itemID int
			var e game.ItemEffect
			var kind string
			if err := r.Scan(&itemID, &e.EffectID, &kind, &e.StatID, &e.Min, &e.Max); err != nil {
				return err
			}
			e.Kind = game.EffectKind(kind)
			if idx, ok := itemIndex[itemID]; ok {
				ds.Items[idx].Effects = append(ds.Items[idx].Effects, e)
			}
			return nil
		}); err != nil {
		return nil, err
	}

	setIndex := map[int]int{}
	if err := scan("SELECT id, level FROM sets ORDER BY id", func(r *sql.Rows) error {
		var set game.Set
		if err := r.Scan(&set.ID, &set.Level); err != nil {
			return err
		}
		set.Name = textsOf(textSet, set.ID)
		setIndex[set.ID] = len(ds.Sets)
		ds.Sets = append(ds.Sets, set)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := scan(`SELECT si.set_id, si.item_id FROM set_items si JOIN items i ON i.id = si.item_id
		ORDER BY si.set_id, i.level, i.id`, func(r *sql.Rows) error {
		var setID, itemID int
		if err := r.Scan(&setID, &itemID); err != nil {
			return err
		}
		if idx, ok := setIndex[setID]; ok {
			ds.Sets[idx].ItemIDs = append(ds.Sets[idx].ItemIDs, itemID)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	for i := range ds.Sets {
		ds.Sets[i].Bonuses = make([][]game.SetBonus, len(ds.Sets[i].ItemIDs)+1)
	}
	if err := scan("SELECT set_id, item_count, effect_id, stat_id, value FROM set_bonuses ORDER BY set_id, item_count, position",
		func(r *sql.Rows) error {
			var setID, count int
			var b game.SetBonus
			if err := r.Scan(&setID, &count, &b.EffectID, &b.StatID, &b.Value); err != nil {
				return err
			}
			idx, ok := setIndex[setID]
			if !ok {
				return nil
			}
			set := &ds.Sets[idx]
			for count >= len(set.Bonuses) {
				set.Bonuses = append(set.Bonuses, nil)
			}
			set.Bonuses[count] = append(set.Bonuses[count], b)
			return nil
		}); err != nil {
		return nil, err
	}
	return ds, nil
}

func passiveSpell(it *game.Item) int {
	if it.Passive == nil {
		return 0
	}
	return it.Passive.SpellID
}
