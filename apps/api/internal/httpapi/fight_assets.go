package httpapi

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"dofusdb/api/internal/fight"
)

var trainingMaps = []int{99222029, 192419850}

// The importer publishes a new immutable directory before switching this pointer.
func (s *Server) assetPack() (string, error) {
	if s.updater == nil {
		return "", fs.ErrNotExist
	}
	var primaryError error
	for _, root := range s.updater.FightAssetsDirs() {
		dir, err := publishedFightPack(root)
		if err == nil {
			return dir, nil
		}
		if primaryError == nil {
			primaryError = err
		}
	}
	return "", primaryError
}

func publishedFightPack(root string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(name, "pack-") || !filepath.IsLocal(name) || strings.ContainsAny(name, `/\`) {
		return "", fs.ErrInvalid
	}
	dir := filepath.Join(root, name)
	marker, err := os.Stat(filepath.Join(dir, "complete.json"))
	if err != nil {
		return "", err
	}
	if !marker.Mode().IsRegular() {
		return "", fs.ErrInvalid
	}
	return dir, nil
}

func (s *Server) fightBoard(id int) (fight.Board, error) {
	dir, err := s.assetPack()
	if err != nil {
		return fight.Board{}, err
	}
	return fightBoardIn(dir, id)
}

func fightBoardIn(dir string, id int) (fight.Board, error) {
	var b fight.Board
	if !slices.Contains(trainingMaps, id) {
		return b, fs.ErrNotExist
	}
	file, err := os.Open(filepath.Join(dir, "maps", strconv.Itoa(id)+".json"))
	if err != nil {
		return b, err
	}
	defer file.Close()
	if err = json.NewDecoder(io.LimitReader(file, 64<<10)).Decode(&b); err != nil {
		return b, err
	}
	if b.MapID != id || b.Width < 1 || b.Height < 1 || b.Width > fight.MaxSide || b.Height > fight.MaxSide || len(b.Blue) < 4 || len(b.Red) < 1 ||
		b.ImageWidth <= 0 || b.ImageHeight <= 0 || b.HalfCellWidth <= 0 || b.HalfCellHeight <= 0 {
		return b, fs.ErrInvalid
	}
	cells := b.Width * b.Height
	for _, group := range [][]int{b.Walls, b.SightWalls, b.Blue, b.Red} {
		if len(group) > cells {
			return b, fs.ErrInvalid
		}
		for _, cell := range group {
			if cell < 0 || cell >= cells {
				return b, fs.ErrInvalid
			}
		}
	}
	// The local fighter takes the inner (attackers') camp, shown blue in the HUD.
	b.Blue, b.Red = b.Red, b.Blue
	starts := map[int][]int{99222029: {62, 125, 124}, 192419850: {116, 52, 62}}[id]
	prioritize := func(cells, preferred []int) []int {
		ordered := make([]int, 0, len(cells))
		for _, v := range preferred {
			if slices.Contains(cells, v) {
				ordered = append(ordered, v)
			}
		}
		for _, v := range cells {
			if !slices.Contains(ordered, v) {
				ordered = append(ordered, v)
			}
		}
		return ordered
	}
	b.Blue = prioritize(b.Blue, starts[:1])
	b.Red = prioritize(b.Red, starts[1:])
	return b, nil
}

func (s *Server) handleFightAssets(w http.ResponseWriter, _ *http.Request) {
	maps := []int{}
	id := ""
	if dir, err := s.assetPack(); err == nil {
		id = filepath.Base(dir)
		for _, mapID := range trainingMaps {
			if _, err := fightBoardIn(dir, mapID); err == nil {
				maps = append(maps, mapID)
			}
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"maps": maps, "pack": id})
}

func (s *Server) serveFightAsset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("path")
	dir, err := s.assetPack()
	ext := strings.ToLower(path.Ext(name))
	if err != nil || !fs.ValidPath(name) || strings.ContainsAny(name, `\:`) || !slices.Contains([]string{".bin", ".dat", ".webp", ".png", ".ttf", ".json", ".ogg", ".woff2"}, ext) {
		http.NotFound(w, r)
		return
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer root.Close()
	f, err := root.Open(filepath.FromSlash(name))
	if os.IsNotExist(err) && path.Base(name) == "skin.json" {
		f, err = root.Open(filepath.FromSlash(name + ".gz"))
		if err == nil {
			defer f.Close()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Add("Vary", "Accept-Encoding")
			if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				w.Header().Set("Content-Encoding", "gzip")
				_, _ = io.Copy(w, f)
			} else {
				decoded, e := gzip.NewReader(f)
				if e != nil {
					http.Error(w, "Invalid asset", http.StatusInternalServerError)
					return
				}
				defer decoded.Close()
				_, _ = io.Copy(w, decoded)
			}
			return
		}
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, path.Base(name), info.ModTime(), f)
}

type creaturePack struct {
	dir     string
	entries map[int]*fight.Creature
}

func (s *Server) fightCreatures() map[int]*fight.Creature {
	dir, err := s.assetPack()
	if err != nil {
		return nil
	}
	if cached := s.creatures.Load(); cached != nil && cached.dir == dir {
		return cached.entries
	}
	data, err := os.ReadFile(filepath.Join(dir, "actors", "creatures.json"))
	if err != nil {
		return nil
	}
	var entries map[int]*fight.Creature
	if json.Unmarshal(data, &entries) != nil {
		return nil
	}
	s.creatures.Store(&creaturePack{dir: dir, entries: entries})
	return entries
}
