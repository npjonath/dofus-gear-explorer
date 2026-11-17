package httpapi

import (
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// A search computes up to twenty stuffs and each weighs about thirty
// kilobytes once evaluated, so they are served a page at a time: the search
// answers with the first page, and the rest is read from the results kept
// here.
const (
	// resultPage is the number of stuffs in a page unless asked otherwise.
	resultPage = 5
	// resultPageMax is the most a page may hold.
	resultPageMax = 20
	// resultSets is how many searches keep their results.
	resultSets = 8
)

type resultSet struct {
	id        string
	solutions []solutionDTO
}

// results keeps the stuffs of the latest searches, oldest first.
type results struct {
	mu   sync.Mutex
	sets []resultSet
}

func (r *results) put(solutions []solutionDTO) string {
	raw := make([]byte, 8)
	_, _ = rand.Read(raw)
	id := hex.EncodeToString(raw)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sets = append(r.sets, resultSet{id: id, solutions: solutions})
	if len(r.sets) > resultSets {
		r.sets = r.sets[len(r.sets)-resultSets:]
	}
	return id
}

func (r *results) get(id string) ([]solutionDTO, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, set := range r.sets {
		if set.id == id {
			return set.solutions, true
		}
	}
	return nil, false
}

// page cuts one page out of a list of stuffs.
func page(solutions []solutionDTO, offset, limit int) []solutionDTO {
	if limit <= 0 {
		limit = resultPage
	}
	limit = min(limit, resultPageMax)
	offset = max(0, min(offset, len(solutions)))
	return solutions[offset:min(offset+limit, len(solutions))]
}

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	solutions, ok := s.results.get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "results_expired", "These results are no longer kept; run the search again.")
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, http.StatusOK, map[string]any{
		"solutions": page(solutions, offset, limit),
		"total":     len(solutions),
	})
}

var gzipWriters = sync.Pool{New: func() any {
	zw, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
	return zw
}}

type gzipResponse struct {
	http.ResponseWriter
	zw *gzip.Writer
}

func (g gzipResponse) Write(b []byte) (int, error) { return g.zw.Write(b) }

// compress gzips the JSON answers of the API for clients that accept it.
// Icons are PNG already and pass through.
func compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		zw := gzipWriters.Get().(*gzip.Writer)
		zw.Reset(w)
		defer func() {
			_ = zw.Close()
			gzipWriters.Put(zw)
		}()
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		next.ServeHTTP(gzipResponse{ResponseWriter: w, zw: zw}, r)
	})
}
