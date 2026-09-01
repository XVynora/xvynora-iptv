package src

import (
	"net/http"
)

func XVynoraIPTVUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/iptv/" && r.URL.Path != "/iptv" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, "html/iptv/index.html")
}
