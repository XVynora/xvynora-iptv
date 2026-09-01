package src

import (
	"net/http"
	"strings"
)

func XVynoraIPTVUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/iptv/" && r.URL.Path != "/iptv" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Prevent accidental routing of nested paths.
	if strings.TrimSuffix(r.URL.Path, "/") != "/iptv" {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, "html/iptv/index.html")
}
