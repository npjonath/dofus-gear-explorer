package httpapi

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// Access decides which callers the API answers. The API runs on the player's
// machine, so any web page open in their browser can send it requests: only
// the app's own pages may use it.
type Access struct {
	// AllowedOrigins are the browser origins allowed to call the API from
	// another origin, like the web app's dev server. "*" allows every origin,
	// for an API deliberately served to the public.
	AllowedOrigins []string
	// LocalOnly rejects requests whose Host is not a loopback name. A page
	// that rebinds its own domain to 127.0.0.1 would otherwise pass as
	// same-origin.
	LocalOnly bool
}

// guard rejects requests sent by foreign web pages and answers CORS for the
// allowed ones. Requests without an Origin header come from the desktop
// shell (whose file:// pages send none), from the API's own pages or from
// tools like curl; browsers always send one on a cross-origin fetch or POST.
func (s *Server) guard(next http.Handler) http.Handler {
	allowed := make(map[string]bool, len(s.access.AllowedOrigins))
	for _, origin := range s.access.AllowedOrigins {
		allowed[strings.TrimSuffix(origin, "/")] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.access.LocalOnly && !IsLoopbackHost(r.Host) {
			writeError(w, http.StatusForbidden, "forbidden_host", "The API only answers requests addressed to this computer.")
			return
		}
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		// Chromium loads fonts and CSS masks from a packaged file:// page in
		// anonymous CORS mode. The artwork routes contain only game resources.
		art := r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/fight-assets/")
		if art && (origin == "" || origin == "null" || origin == "file://") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			next.ServeHTTP(w, r)
			return
		}
		switch {
		case origin == "":
		case allowed["*"] || allowed[origin]:
			h := w.Header()
			if allowed["*"] {
				h.Set("Access-Control-Allow-Origin", "*")
			} else {
				h.Set("Access-Control-Allow-Origin", origin)
			}
			h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type")
			h.Set("Access-Control-Max-Age", "600")
		case sameOrigin(origin, r.Host):
		default:
			writeError(w, http.StatusForbidden, "forbidden_origin", "Requests from this page are not allowed.")
			return
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	return err == nil && u.Host != "" && strings.EqualFold(u.Host, host)
}

// IsLoopbackHost reports whether host, with or without a port, names this
// computer: localhost, a *.localhost name or a loopback address.
func IsLoopbackHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSuffix(strings.Trim(host, "[]"), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
