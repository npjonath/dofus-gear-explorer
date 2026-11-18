// Package httpapi exposes the catalog, the build engine, the solver and the
// data updater over HTTP.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"dofusdb/api/internal/engine"
	"dofusdb/api/internal/game"
	"dofusdb/api/internal/solver"
	"dofusdb/api/internal/store"
	"dofusdb/api/internal/updater"
)

type Server struct {
	store   *store.Store
	updater *updater.Updater
	log     *slog.Logger
	// engine is swapped atomically when a new release is imported.
	engine  atomic.Pointer[engine.Engine]
	access  Access
	webDir  string
	results results
}

func New(st *store.Store, log *slog.Logger, access Access, webDir string) *Server {
	return &Server{store: st, log: log, access: access, webDir: webDir}
}

func (s *Server) SetUpdater(u *updater.Updater) { s.updater = u }

// SetDataset makes a freshly imported dataset the one being served.
func (s *Server) SetDataset(ds *game.Dataset) {
	s.engine.Store(engine.New(game.NewCatalog(ds)))
	s.log.Info("dataset loaded", "version", ds.Version, "items", len(ds.Items), "sets", len(ds.Sets))
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	mux.HandleFunc("GET /api/data/status", s.handleStatus)
	mux.HandleFunc("POST /api/data/check", s.handleCheck)
	mux.HandleFunc("POST /api/data/update", s.handleUpdate)
	mux.HandleFunc("GET /api/meta", s.withData(s.handleMeta))
	mux.HandleFunc("GET /api/items", s.withData(s.handleItems))
	mux.HandleFunc("GET /api/items/{id}", s.withData(s.handleItem))
	mux.HandleFunc("GET /api/sets/{id}", s.withData(s.handleSet))
	mux.HandleFunc("POST /api/builds/evaluate", s.withData(s.handleEvaluate))
	mux.HandleFunc("POST /api/builds/damage", s.withData(s.handleDamage))
	mux.HandleFunc("POST /api/solver/search", s.withData(s.handleSolve))
	mux.HandleFunc("GET /api/solver/results/{id}", s.handleResults)
	mux.HandleFunc("GET /images/items/{file}", s.handleIcon)
	if s.webDir != "" {
		mux.Handle("/", spaHandler(s.webDir))
	}
	return s.guard(compress(mux))
}

func spaHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(clean); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		files.ServeHTTP(w, r)
	})
}

type apiError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, apiError{Error: code, Message: message})
}

type dataHandler func(w http.ResponseWriter, r *http.Request, eng *engine.Engine, lang string)

func (s *Server) withData(h dataHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		eng := s.engine.Load()
		if eng == nil {
			writeError(w, http.StatusServiceUnavailable, "data_not_ready", "Game data has not been imported yet.")
			return
		}
		lang := r.URL.Query().Get("lang")
		known := false
		for _, l := range eng.Catalog.Languages {
			if l == lang {
				known = true
			}
		}
		if !known {
			lang = "fr"
			if len(eng.Catalog.Languages) > 0 && !contains(eng.Catalog.Languages, "fr") {
				lang = eng.Catalog.Languages[0]
			}
		}
		h(w, r, eng, lang)
	}
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.updater.Status())
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	st, err := s.updater.Check(r.Context())
	if err != nil && !errors.Is(err, updater.ErrBusy) {
		writeError(w, http.StatusBadGateway, "check_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if err := s.updater.StartUpdate(""); err != nil && !errors.Is(err, updater.ErrBusy) {
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, s.updater.Status())
}

func (s *Server) handleIcon(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	id, err := strconv.Atoi(strings.TrimSuffix(name, ".png"))
	if err != nil || !strings.HasSuffix(name, ".png") {
		http.NotFound(w, r)
		return
	}
	// Icons change when a release or another icon size is imported: the
	// browser keeps them but asks each time whether they are still current.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, filepath.Join(s.updater.ImagesDir(), strconv.Itoa(id)+".png"))
}

type statDTO struct {
	ID         int    `json:"id"`
	Key        string `json:"key"`
	Label      string `json:"label"`
	Unit       string `json:"unit"`
	CategoryID int    `json:"categoryId"`
	// ForgeMax is the highest value forgemagic can put on a line of this
	// stat; zero when the stat cannot be forged.
	ForgeMax int `json:"forgeMax"`
}

type runeDTO struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	StatID int    `json:"statId"`
	Value  int    `json:"value"`
	// Density is the weight the rune counts for on its line.
	Density int    `json:"density"`
	Text    string `json:"text"`
}

type dracoRuneDTO struct {
	Key string `json:"key"`
	// Kind is "element" (stat and damage of one element), "resistance" or "power".
	Kind string `json:"kind"`
	// Target is the element an "element" rune converts to.
	Target string `json:"target,omitempty"`
}

type forgeMetaDTO struct {
	Slots       []game.Slot                    `json:"slots"`
	LimitedExos []int                          `json:"limitedExos"`
	Runes       []runeDTO                      `json:"runes"`
	DracoRunes  []dracoRuneDTO                 `json:"dracoRunes"`
	Elements    []string                       `json:"elements"`
	ElementIDs  map[string]engine.ElementStats `json:"elementIds"`
}

type categoryDTO struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	StatIDs []int  `json:"statIds"`
}

type typeDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slot string `json:"slot"`
}

type metaDTO struct {
	Version      string          `json:"version"`
	Languages    []string        `json:"languages"`
	MaxLevel     int             `json:"maxLevel"`
	Slots        []game.Slot     `json:"slots"`
	Positions    []game.Position `json:"positions"`
	Categories   []categoryDTO   `json:"categories"`
	Stats        []statDTO       `json:"stats"`
	Types        []typeDTO       `json:"types"`
	PrimaryStats []primaryDTO    `json:"primaryStats"`
	ItemCount    int             `json:"itemCount"`
	SetCount     int             `json:"setCount"`
	Forge        forgeMetaDTO    `json:"forge"`
	// Breeds are the character classes; Spells every spell a character can
	// cast, class by class, then the common ones (breed zero).
	Breeds []breedDTO `json:"breeds"`
	Spells []spellDTO `json:"spells"`
	// Prysmaradite names the item type that only fits one position.
	Prysmaradite prysmaraditeDTO `json:"prysmaradite"`
}

type breedDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type spellDTO struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	BreedID   int    `json:"breedId"`
	VariantOf int    `json:"variantOf"`
	// Damage says the spell deals direct damage at some grade.
	Damage bool `json:"damage"`
	// Elements lists the elements the spell hits in at once, at some grade:
	// "earth", "fire", "water", "air", "neutral", or "best" for a line that
	// follows the character's best element.
	Elements []string `json:"elements"`
}

var elementNames = map[int]string{
	game.ElementNeutral: "neutral", game.ElementEarth: "earth", game.ElementFire: "fire",
	game.ElementWater: "water", game.ElementAir: "air", game.ElementBest: "best",
}

type prysmaraditeDTO struct {
	TypeID   int    `json:"typeId"`
	Position string `json:"position"`
}

type primaryDTO struct {
	Key    string `json:"key"`
	StatID int    `json:"statId"`
}

var primaryIDs = map[string]int{
	"vitality": game.StatVitality, "wisdom": game.StatWisdom, "strength": game.StatStrength,
	"intelligence": game.StatIntel, "chance": game.StatChance, "agility": game.StatAgility,
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request, eng *engine.Engine, lang string) {
	cat := eng.Catalog
	meta := metaDTO{
		Version: cat.Version, Languages: cat.Languages, MaxLevel: engine.MaxLevel,
		Slots: game.Slots, Positions: game.Positions,
		ItemCount: len(cat.Items), SetCount: len(cat.Sets),
		Prysmaradite: prysmaraditeDTO{TypeID: game.TypePrysmaradite, Position: game.PrysmaraditePosition},
	}
	stats := append([]game.Stat{}, cat.Stats...)
	sort.SliceStable(stats, func(a, b int) bool { return stats[a].Order < stats[b].Order })
	byCategory := map[int][]int{}
	for _, st := range stats {
		meta.Stats = append(meta.Stats, statDTO{
			ID: st.ID, Key: st.Key, Label: st.Label.Get(lang), Unit: st.Unit, CategoryID: st.CategoryID,
			ForgeMax: st.ForgeMax(),
		})
		byCategory[st.CategoryID] = append(byCategory[st.CategoryID], st.ID)
	}
	for _, c := range cat.Categories {
		if len(byCategory[c.ID]) == 0 {
			continue
		}
		meta.Categories = append(meta.Categories, categoryDTO{ID: c.ID, Name: c.Name.Get(lang), StatIDs: byCategory[c.ID]})
	}
	for _, t := range cat.Types {
		meta.Types = append(meta.Types, typeDTO{ID: t.ID, Name: t.Name.Get(lang), Slot: string(t.Slot)})
	}
	for _, key := range engine.PrimaryStats {
		meta.PrimaryStats = append(meta.PrimaryStats, primaryDTO{Key: key, StatID: primaryIDs[key]})
	}

	meta.Breeds, meta.Spells = []breedDTO{}, []spellDTO{}
	breedIDs := []int{}
	for _, b := range cat.Breeds {
		meta.Breeds = append(meta.Breeds, breedDTO{ID: b.ID, Name: b.Name.Get(lang)})
		breedIDs = append(breedIDs, b.ID)
	}
	for _, breedID := range append(breedIDs, 0) {
		for _, sp := range cat.SpellsByBreed[breedID] {
			dto := spellDTO{ID: sp.ID, Name: sp.Name.Get(lang), BreedID: sp.BreedID, VariantOf: sp.VariantOf, Elements: []string{}}
			for _, level := range sp.Levels {
				dto.Damage = dto.Damage || len(level.Lines) > 0
				for _, line := range level.Lines {
					if name := elementNames[line.Element]; line.Delay == 0 && name != "" && !slices.Contains(dto.Elements, name) {
						dto.Elements = append(dto.Elements, name)
					}
				}
			}
			meta.Spells = append(meta.Spells, dto)
		}
	}

	meta.Forge = forgeMetaDTO{
		LimitedExos: engine.LimitedExos, Elements: engine.Elements, ElementIDs: engine.ElementTable,
		Runes: []runeDTO{},
	}
	for _, slot := range game.Slots {
		if engine.ForgeableSlots[slot] {
			meta.Forge.Slots = append(meta.Forge.Slots, slot)
		}
	}
	for _, rn := range cat.Runes {
		meta.Forge.Runes = append(meta.Forge.Runes, runeDTO{
			ID: rn.ID, Name: rn.Name.Get(lang), StatID: rn.StatID, Value: rn.Value, Density: rn.Density,
			Text: eng.StatText(rn.StatID, rn.Value, lang),
		})
	}
	for _, key := range engine.DracoRunes {
		dto := dracoRuneDTO{Key: key, Kind: "element", Target: engine.DracoTargets[key]}
		switch key {
		case engine.DracoIvory:
			dto.Kind = "resistance"
		case engine.DracoEbony:
			dto.Kind = "power"
		}
		meta.Forge.DracoRunes = append(meta.Forge.DracoRunes, dto)
	}
	writeJSON(w, http.StatusOK, meta)
}

type effectDTO struct {
	Kind    string `json:"kind"`
	StatID  *int   `json:"statId"`
	Element *int   `json:"element"`
	Min     int    `json:"min"`
	Max     int    `json:"max"`
	Text    string `json:"text"`
}

type setRefDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type itemDTO struct {
	ID          int              `json:"id"`
	Name        string           `json:"name"`
	Level       int              `json:"level"`
	TypeID      int              `json:"typeId"`
	TypeName    string           `json:"typeName"`
	Slot        string           `json:"slot"`
	Image       string           `json:"image"`
	Set         *setRefDTO       `json:"set"`
	Effects     []effectDTO      `json:"effects"`
	Condition   string           `json:"condition"`
	Weapon      *game.WeaponInfo `json:"weapon"`
	Passive     *passiveDTO      `json:"passive"`
	Forgeable   bool             `json:"forgeable"`
	Description string           `json:"description,omitempty"`
}

type passiveDTO struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// iconQuery tells apart the icon addresses of today from those served
// before icons were revalidated: browsers kept the old ones for a day.
const iconQuery = "?r=2"

func toItemDTO(eng *engine.Engine, it *game.Item, lang string, withDescription bool) itemDTO {
	cat := eng.Catalog
	dto := itemDTO{
		ID: it.ID, Name: it.Name.Get(lang), Level: it.Level, TypeID: it.TypeID,
		Slot: string(it.Slot), Image: "/images/items/" + strconv.Itoa(it.IconID) + ".png" + iconQuery,
		Effects: make([]effectDTO, 0, len(it.Effects)), Weapon: it.Weapon,
		Condition: eng.Criteria(it.ID).Text(cat, lang),
		Forgeable: engine.ForgeableSlots[it.Slot],
	}
	if it.Passive != nil {
		dto.Passive = &passiveDTO{Name: it.Passive.Name.Get(lang), Description: it.Passive.Description.Get(lang)}
	}
	if t := cat.TypeByID[it.TypeID]; t != nil {
		dto.TypeName = t.Name.Get(lang)
	}
	if set := cat.SetByID[it.SetID]; set != nil {
		dto.Set = &setRefDTO{ID: set.ID, Name: set.Name.Get(lang)}
	}
	for _, e := range it.Effects {
		line := effectDTO{Kind: string(e.Kind), Min: e.Min, Max: e.Max}
		if def := cat.EffectByID[e.EffectID]; def != nil {
			line.Text = game.RenderEffect(def.Template.Get(lang), e.Min, e.Max)
			if e.Kind == game.EffectWeapon && def.Element >= 0 {
				element := def.Element
				line.Element = &element
			}
		}
		if e.Kind == game.EffectStat {
			statID := e.StatID
			line.StatID = &statID
		}
		dto.Effects = append(dto.Effects, line)
	}
	if withDescription {
		dto.Description = it.Description.Get(lang)
	}
	return dto
}

func intList(raw string) []int {
	var out []int
	for _, part := range strings.Split(raw, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func (s *Server) handleItems(w http.ResponseWriter, r *http.Request, eng *engine.Engine, lang string) {
	q := r.URL.Query()
	cat := eng.Catalog

	// ids=1,2,3 returns exactly those items, to hydrate saved builds.
	if raw := q.Get("ids"); raw != "" {
		items := []itemDTO{}
		for _, id := range intList(raw) {
			if it := cat.ItemByID[id]; it != nil {
				items = append(items, toItemDTO(eng, it, lang, false))
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
		return
	}

	query := store.ItemQuery{
		Lang:           lang,
		Search:         q.Get("q"),
		TypeIDs:        intList(q.Get("types")),
		ExcludeTypeIDs: intList(q.Get("excludeTypes")),
		Sort:           q.Get("sort"),
		Desc:           q.Get("order") == "desc",
	}
	if raw := q.Get("slots"); raw != "" {
		query.Slots = strings.Split(raw, ",")
	}
	query.LevelMin, _ = strconv.Atoi(q.Get("levelMin"))
	query.LevelMax, _ = strconv.Atoi(q.Get("levelMax"))
	query.Limit, _ = strconv.Atoi(q.Get("limit"))
	query.Offset, _ = strconv.Atoi(q.Get("offset"))
	query.SetID, _ = strconv.Atoi(q.Get("setId"))
	query.Passive = q.Get("passive") == "true"
	if raw := q.Get("inSet"); raw != "" {
		inSet := raw == "true"
		query.InSet = &inSet
	}
	if strings.HasPrefix(query.Sort, "stat:") {
		id, err := strconv.Atoi(strings.TrimPrefix(query.Sort, "stat:"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_sort", "sort must be level, name or stat:<id>")
			return
		}
		query.Sort, query.SortStatID = "stat", id
	}
	// stat=<id>:<gte|lte>:<value>, repeatable.
	for _, raw := range q["stat"] {
		parts := strings.Split(raw, ":")
		if len(parts) != 3 {
			writeError(w, http.StatusBadRequest, "bad_filter", "stat filters look like 11:gte:300")
			return
		}
		id, err1 := strconv.Atoi(parts[0])
		value, err2 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || (parts[1] != "gte" && parts[1] != "lte") {
			writeError(w, http.StatusBadRequest, "bad_filter", "stat filters look like 11:gte:300")
			return
		}
		query.Stats = append(query.Stats, store.StatFilter{StatID: id, Op: parts[1], Value: value})
	}

	ids, total, err := s.store.QueryItems(r.Context(), query)
	if err != nil {
		s.log.Error("item query failed", "error", err)
		writeError(w, http.StatusInternalServerError, "query_failed", "The item search failed.")
		return
	}
	items := make([]itemDTO, 0, len(ids))
	for _, id := range ids {
		if it := cat.ItemByID[id]; it != nil {
			items = append(items, toItemDTO(eng, it, lang, false))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

type setDTO struct {
	ID      int                `json:"id"`
	Name    string             `json:"name"`
	Level   int                `json:"level"`
	Items   []itemDTO          `json:"items"`
	Bonuses []setBonusLevelDTO `json:"bonuses"`
}

type setBonusLevelDTO struct {
	Count int                 `json:"count"`
	Lines []engine.EffectLine `json:"lines"`
}

func toSetDTO(eng *engine.Engine, set *game.Set, lang string) setDTO {
	dto := setDTO{ID: set.ID, Name: set.Name.Get(lang), Level: set.Level, Items: []itemDTO{}, Bonuses: []setBonusLevelDTO{}}
	for _, id := range set.ItemIDs {
		if it := eng.Catalog.ItemByID[id]; it != nil {
			dto.Items = append(dto.Items, toItemDTO(eng, it, lang, false))
		}
	}
	for count := 2; count <= len(set.ItemIDs) && count < len(set.Bonuses); count++ {
		dto.Bonuses = append(dto.Bonuses, setBonusLevelDTO{Count: count, Lines: eng.SetBonusLines(set, count, lang)})
	}
	return dto
}

func (s *Server) handleItem(w http.ResponseWriter, r *http.Request, eng *engine.Engine, lang string) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	it := eng.Catalog.ItemByID[id]
	if it == nil {
		writeError(w, http.StatusNotFound, "not_found", "Unknown item.")
		return
	}
	out := struct {
		itemDTO
		SetDetail *setDTO `json:"setDetail"`
	}{itemDTO: toItemDTO(eng, it, lang, true)}
	if set := eng.Catalog.SetByID[it.SetID]; set != nil {
		detail := toSetDTO(eng, set, lang)
		out.SetDetail = &detail
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSet(w http.ResponseWriter, r *http.Request, eng *engine.Engine, lang string) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	set := eng.Catalog.SetByID[id]
	if set == nil {
		writeError(w, http.StatusNotFound, "not_found", "Unknown set.")
		return
	}
	writeJSON(w, http.StatusOK, toSetDTO(eng, set, lang))
}

type evaluationDTO struct {
	engine.Result
	Items map[string]itemDTO `json:"items"`
}

func evaluate(eng *engine.Engine, b engine.Build, lang string) evaluationDTO {
	out := evaluationDTO{Result: eng.Evaluate(b, lang), Items: map[string]itemDTO{}}
	for position, id := range b.Items {
		if it := eng.Catalog.ItemByID[id]; it != nil && game.PositionIndex(position) >= 0 {
			out.Items[position] = toItemDTO(eng, it, lang, false)
		}
	}
	return out
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "The request body is not valid JSON.")
		return false
	}
	return true
}

func (s *Server) handleEvaluate(w http.ResponseWriter, r *http.Request, eng *engine.Engine, lang string) {
	var b engine.Build
	if !decodeBody(w, r, &b) {
		return
	}
	writeJSON(w, http.StatusOK, evaluate(eng, b, lang))
}

// damageRequest is a stuff plus what to compute its attacks for.
type damageRequest struct {
	engine.Build
	// BreedID is the character's class; zero leaves the class spells out.
	BreedID int `json:"breedId"`
	// Range is "melee", "ranged" or empty to follow each attack's reach.
	Range string `json:"range"`
	// Mastery counts the weapon with its mastery; it does unless set to false.
	Mastery *bool `json:"mastery"`
}

type attackDTO struct {
	engine.Attack
	Name      string `json:"name"`
	BreedID   int    `json:"breedId"`
	VariantOf int    `json:"variantOf"`
}

type damageDTO struct {
	Weapon *attackDTO  `json:"weapon"`
	Spells []attackDTO `json:"spells"`
}

func (s *Server) handleDamage(w http.ResponseWriter, r *http.Request, eng *engine.Engine, lang string) {
	var req damageRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Range != engine.RangeAuto && req.Range != engine.RangeMelee && req.Range != engine.RangeRanged {
		writeError(w, http.StatusBadRequest, "bad_request", "range must be melee, ranged or empty.")
		return
	}
	cat := eng.Catalog
	res := eng.Evaluate(req.Build, lang)
	stat := func(id int) int { return res.Stats[id] }
	out := damageDTO{Spells: []attackDTO{}}
	if it := cat.ItemByID[req.Items["weapon"]]; it != nil {
		opts := engine.WeaponOptions{Range: req.Range, Mastery: req.Mastery == nil || *req.Mastery}
		if mods := req.Mods["weapon"]; mods != nil {
			opts.Element = mods.Element
		}
		if a := engine.WeaponAttack(it, opts, stat); a != nil {
			out.Weapon = &attackDTO{Attack: *a, Name: it.Name.Get(lang)}
		}
	}
	breeds := []int{0}
	if cat.BreedByID[req.BreedID] != nil {
		breeds = []int{req.BreedID, 0}
	}
	for _, breedID := range breeds {
		for _, sp := range cat.SpellsByBreed[breedID] {
			if a := engine.SpellAttack(sp, res.Level, req.Range, stat); a != nil {
				out.Spells = append(out.Spells, attackDTO{Attack: *a, Name: sp.Name.Get(lang), BreedID: sp.BreedID, VariantOf: sp.VariantOf})
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type solutionDTO struct {
	solver.Solution
	Evaluation evaluationDTO `json:"evaluation"`
}

func (s *Server) handleSolve(w http.ResponseWriter, r *http.Request, eng *engine.Engine, lang string) {
	var req solver.Request
	if !decodeBody(w, r, &req) {
		return
	}
	if len(req.Constraints) > 24 || len(req.Maximize) > 24 {
		writeError(w, http.StatusBadRequest, "too_many_criteria", "Use at most 24 constraints and 24 stats to maximize.")
		return
	}
	for _, c := range req.Constraints {
		if c.Value < 0 {
			writeError(w, http.StatusBadRequest, "negative_value", "A condition's value must be 0 or more.")
			return
		}
	}
	for _, g := range req.Damage {
		if g.Value < 1 {
			writeError(w, http.StatusBadRequest, "negative_value", "A damage to reach must be 1 or more.")
			return
		}
	}
	started := time.Now()
	resp, err := solver.Solve(r.Context(), eng, req)
	if err != nil {
		if errors.Is(err, solver.ErrNothingToSearch) {
			writeError(w, http.StatusBadRequest, "nothing_to_search", err.Error())
			return
		}
		if errors.Is(err, solver.ErrBadDamageGoal) {
			writeError(w, http.StatusBadRequest, "bad_damage_goal", err.Error())
			return
		}
		if r.Context().Err() != nil {
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	// The stuffs that meet every condition come first; when some do, only
	// those are listed.
	feasible := 0
	for _, sol := range resp.Solutions {
		if sol.Feasible {
			feasible++
		}
	}
	listed := resp.Solutions
	if feasible > 0 {
		listed = listed[:0:0]
		for _, sol := range resp.Solutions {
			if sol.Feasible {
				listed = append(listed, sol)
			}
		}
	}
	solutions := make([]solutionDTO, 0, len(listed))
	for _, sol := range listed {
		build := engine.Build{Character: sol.Character, Items: sol.Items, Mods: sol.Mods}
		solutions = append(solutions, solutionDTO{Solution: sol, Evaluation: evaluate(eng, build, lang)})
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	s.log.Info("solver run", "constraints", len(req.Constraints), "solutions", len(solutions),
		"explored", resp.Explored, "elapsed", time.Since(started).Round(time.Millisecond))
	// The first page goes with the answer; the others are asked for by id.
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         s.results.put(solutions),
		"solutions":  page(solutions, 0, limit),
		"total":      len(solutions),
		"feasible":   feasible,
		"explored":   resp.Explored,
		"candidates": resp.Candidates,
		"elapsedMs":  resp.ElapsedMs,
		"unplaced":   resp.Unplaced,
	})
}
