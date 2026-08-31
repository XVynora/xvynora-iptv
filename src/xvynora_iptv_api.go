package src

import (
	"encoding/json"
	"net/http"
	"strings"
)

func XVynoraIPTVAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/xvynora/api/")

	write := func(value interface{}) {
		_ = json.NewEncoder(w).Encode(value)
	}

	switch path {
	case "", "sources":
		write(map[string]interface{}{
			"status":  true,
			"sources": xvynoraIPTVSources,
		})
		return

	case "channels":
		sourceID := r.URL.Query().Get("source")
		if sourceID == "" {
			sourceID = "uk"
		}

		source, ok := xvynoraIPTVGetSource(sourceID)
		if !ok {
			write(map[string]interface{}{
				"status": false,
				"error":  "unknown source",
			})
			return
		}

		data, err := xvynoraIPTVDownload(source.URL)
		if err != nil {
			write(map[string]interface{}{
				"status": false,
				"error":  err.Error(),
			})
			return
		}

		channels := xvynoraIPTVParseM3U(data)

		search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
		category := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("category")))

		filtered := make([]XVynoraIPTVChannel, 0)

		for _, channel := range channels {
			if search != "" && !strings.Contains(strings.ToLower(channel.Name), search) {
				continue
			}

			if category != "" && strings.ToLower(channel.Category) != category {
				continue
			}

			filtered = append(filtered, channel)
		}

		write(map[string]interface{}{
			"status":   true,
			"source":   source,
			"count":    len(filtered),
			"channels": filtered,
		})
		return

	case "import":
		id := r.URL.Query().Get("source")
		if id == "" {
			id = "uk"
		}

		err := xvynoraIPTVImport(id)

		if err != nil {
			write(map[string]interface{}{
				"status": false,
				"error":  err.Error(),
			})
			return
		}

		write(map[string]interface{}{
			"status": true,
			"source": id,
		})
		return

	case "update":
		id := r.URL.Query().Get("source")
		if id == "" {
			id = "uk"
		}

		err := xvynoraIPTVUpdate(id)

		if err != nil {
			write(map[string]interface{}{
				"status": false,
				"error":  err.Error(),
			})
			return
		}

		write(map[string]interface{}{
			"status": true,
			"source": id,
		})
		return

	case "remove":
		id := r.URL.Query().Get("source")
		if id == "" {
			write(map[string]interface{}{
				"status": false,
				"error":  "source required",
			})
			return
		}

		err := xvynoraIPTVRemove(id)

		if err != nil {
			write(map[string]interface{}{
				"status": false,
				"error":  err.Error(),
			})
			return
		}

		write(map[string]interface{}{
			"status": true,
			"source": id,
		})
		return

	case "status":
		result := make([]map[string]interface{}, 0)

		for _, source := range xvynoraIPTVSources {
			result = append(result, xvynoraIPTVProviderStatus(source.ID))
		}

		write(map[string]interface{}{
			"status":  true,
			"sources": result,
		})
		return

	default:
		http.NotFound(w, r)
	}
}
