package src

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type XVynoraIPTVProgram struct {
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	Start       string  `json:"start"`
	Stop        string  `json:"stop"`
	Progress    float64 `json:"progress,omitempty"`
	Source      string  `json:"source,omitempty"`
}

type XVynoraIPTVStreamView struct {
	URL          string `json:"url"`
	Feed         string `json:"feed,omitempty"`
	Title        string `json:"title,omitempty"`
	Source       string `json:"source,omitempty"`
	Provider     string `json:"provider,omitempty"`
	Country      string `json:"country,omitempty"`
	Region       string `json:"region,omitempty"`
	Language     string `json:"language,omitempty"`
	Quality      string `json:"quality,omitempty"`
	Availability string `json:"availability,omitempty"`
	Referrer     string `json:"referrer,omitempty"`
	UserAgent    string `json:"user_agent,omitempty"`
	Health       string `json:"health"`
}

type XVynoraIPTVChannelView struct {
	ID           string                  `json:"id"`
	Title        string                  `json:"title"`
	Name         string                  `json:"name"`
	Stream       string                  `json:"stream"`
	ThreadfinURL string                  `json:"threadfin_url,omitempty"`
	Logo         string                  `json:"logo,omitempty"`
	Group        string                  `json:"group"`
	Category     string                  `json:"category"`
	Country      string                  `json:"country,omitempty"`
	Language     string                  `json:"language,omitempty"`
	Quality      string                  `json:"quality,omitempty"`
	Health       string                  `json:"health"`
	Status       string                  `json:"status"`
	Source       string                  `json:"source,omitempty"`
	Provider     string                  `json:"provider,omitempty"`
	Region       string                  `json:"region,omitempty"`
	TvgID        string                  `json:"tvgId,omitempty"`
	EPGID        string                  `json:"epg_id,omitempty"`
	SkySports    bool                    `json:"sky_sports,omitempty"`
	Description  string                  `json:"description,omitempty"`
	Now          *XVynoraIPTVProgram     `json:"now,omitempty"`
	Next         *XVynoraIPTVProgram     `json:"next,omitempty"`
	Metadata     map[string]interface{}  `json:"metadata,omitempty"`
	StreamURLs   []string                `json:"stream_urls,omitempty"`
	EPGSources   []string                `json:"epg_sources,omitempty"`
	AltNames     []string                `json:"alt_names,omitempty"`
	Network      string                  `json:"network,omitempty"`
	LastChecked  string                  `json:"last_checked,omitempty"`
	Streams      []XVynoraIPTVStreamView `json:"streams,omitempty"`
}

type xvynoraIPTVSnapshot struct {
	builtAt  time.Time
	allCount int
	active   []XVynoraIPTVChannelView
	groups   []string
}

var (
	xvynoraIPTVSnapshotMu sync.RWMutex
	xvynoraIPTVSnap       xvynoraIPTVSnapshot
)

func XVynoraIPTVAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		xvynoraWriteJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"status": false, "error": "method not allowed"})
		return
	}

	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/xvynora/api/"), "/")
	if path == "" {
		path = "status"
	}

	switch path {
	case "sources", "status":
		xvynoraWriteJSON(w, http.StatusOK, map[string]interface{}{
			"status":           true,
			"sources":          xvynoraIPTVSourceStatus(),
			"scan_in_progress": System.ScanInProgress == 1,
			"streams": map[string]int{
				"all":    len(Data.Streams.All),
				"active": len(Data.Streams.Active),
				"xepg":   len(Data.XEPG.Channels),
			},
		})

	case "catalogue", "catalogue/status":
		cache, err := xvynoraIPTVCatalogueEntries()
		if err != nil {
			xvynoraWriteJSON(w, http.StatusOK, map[string]interface{}{
				"status":    true,
				"available": false,
				"error":     err.Error(),
			})
			return
		}
		xvynoraWriteJSON(w, http.StatusOK, map[string]interface{}{
			"status":       true,
			"available":    true,
			"count":        len(cache.Channels),
			"last_refresh": cache.LastRefresh,
		})

	case "catalogue/refresh":
		if r.Method != http.MethodPost {
			xvynoraWriteJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"status": false, "error": "method not allowed"})
			return
		}
		if err := xvynoraIPTVRefreshCatalogue(); err != nil {
			xvynoraWriteJSON(w, http.StatusOK, map[string]interface{}{"status": false, "error": err.Error()})
			return
		}
		cache, _ := xvynoraIPTVCatalogueEntries()
		xvynoraWriteJSON(w, http.StatusOK, map[string]interface{}{
			"status":       true,
			"count":        len(cache.Channels),
			"last_refresh": cache.LastRefresh,
		})

	case "categories", "countries", "languages":
		xvynoraWriteJSON(w, http.StatusOK, map[string]interface{}{"status": true, "values": xvynoraIPTVDistinctValues(path)})

	case "health":
		counts := map[string]int{"unknown": 0, "probing": 0, "online": 0, "buffering": 0, "failed": 0}
		for _, channel := range xvynoraIPTVChannels(false) {
			state := channel.Status
			if _, ok := counts[state]; !ok {
				state = "unknown"
			}
			counts[state]++
		}
		xvynoraWriteJSON(w, http.StatusOK, map[string]interface{}{"status": true, "counts": counts})

	case "channels":
		channels := xvynoraIPTVFilterChannels(xvynoraIPTVChannels(false), r)
		offset := xvynoraIPTVPositiveInt(r.URL.Query().Get("offset"))
		total := len(channels)
		if offset > total {
			offset = total
		}
		channels = channels[offset:]
		if limit := xvynoraIPTVPositiveInt(r.URL.Query().Get("limit")); limit > 0 && len(channels) > limit {
			channels = channels[:limit]
		}
		xvynoraWriteJSON(w, http.StatusOK, map[string]interface{}{
			"status":           true,
			"count":            len(channels),
			"total":            total,
			"offset":           offset,
			"channels":         channels,
			"groups":           xvynoraIPTVSnap.groups,
			"scan_in_progress": System.ScanInProgress == 1,
		})

	case "epg":
		id := strings.TrimSpace(r.URL.Query().Get("channel"))
		channel, ok := xvynoraIPTVFindChannel(id)
		if !ok {
			xvynoraWriteJSON(w, http.StatusNotFound, map[string]interface{}{"status": false, "error": "channel not found"})
			return
		}
		now, next := xvynoraIPTVNowNext(channel)
		xvynoraWriteJSON(w, http.StatusOK, map[string]interface{}{"status": true, "channel": id, "now": now, "next": next, "sources": channel.EPGSources})

	case "probe":
		channel, ok := xvynoraIPTVFindChannel(r.URL.Query().Get("channel"))
		if !ok {
			xvynoraWriteJSON(w, http.StatusNotFound, map[string]interface{}{"status": false, "health": "unknown", "error": "channel not found"})
			return
		}
		health, err := xvynoraIPTVProbeURL(channel.Stream)
		result := map[string]interface{}{"status": err == nil, "health": health, "channel": channel.ID}
		if err != nil {
			result["error"] = err.Error()
		}
		xvynoraWriteJSON(w, http.StatusOK, result)

	case "import":
		id, playlistURL, epgURL, name := xvynoraIPTVRequestSource(r)
		data, err := xvynoraIPTVImportSource(id, playlistURL, epgURL, name)
		xvynoraWriteMutation(w, data, err)

	case "update":
		id, _, _, _ := xvynoraIPTVRequestSource(r)
		xvynoraWriteMutation(w, nil, xvynoraIPTVUpdateSource(id))

	case "remove":
		id, _, _, _ := xvynoraIPTVRequestSource(r)
		xvynoraWriteMutation(w, nil, xvynoraIPTVRemoveSource(id))

	case "upload":
		data, err := xvynoraIPTVUpload(r)
		xvynoraWriteMutation(w, data, err)

	default:
		xvynoraWriteJSON(w, http.StatusNotFound, map[string]interface{}{"status": false, "error": "not found"})
	}
}

func xvynoraIPTVUpload(r *http.Request) (interface{}, error) {
	if r.Method != http.MethodPost {
		return nil, errors.New("method not allowed")
	}

	var body struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.Content) == "" {
		return nil, errors.New("empty playlist")
	}
	if !strings.Contains(body.Content, "#EXTM3U") {
		return nil, errors.New("invalid M3U playlist")
	}

	if err := os.MkdirAll(System.Folder.Data, 0755); err != nil {
		return nil, err
	}

	sourcePath := filepath.Join(System.Folder.Data, "xvynora-upload-"+randomString(10)+".m3u")
	if err := os.WriteFile(sourcePath, []byte(body.Content), 0644); err != nil {
		return nil, err
	}

	provider, err := xvynoraIPTVAddProvider("m3u", sourcePath, firstNonEmpty(body.Name, "XVynora Upload"))
	if err != nil {
		_ = os.Remove(sourcePath)
		return nil, err
	}
	if err := buildDatabaseDVR(); err != nil {
		return provider, err
	}
	buildXEPG(false)
	return provider, nil
}

func xvynoraWriteJSON(w http.ResponseWriter, status int, value interface{}) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func xvynoraWriteMutation(w http.ResponseWriter, data interface{}, err error) {
	if err != nil {
		xvynoraWriteJSON(w, http.StatusOK, map[string]interface{}{"status": false, "error": err.Error()})
		return
	}
	xvynoraIPTVInvalidateSnapshot()
	xvynoraWriteJSON(w, http.StatusOK, map[string]interface{}{"status": true, "data": data})
}

func xvynoraIPTVRequestSource(r *http.Request) (sourceID, playlistURL, epgURL, name string) {
	sourceID = strings.TrimSpace(r.URL.Query().Get("source"))
	playlistURL = strings.TrimSpace(r.URL.Query().Get("url"))
	epgURL = strings.TrimSpace(r.URL.Query().Get("epg"))
	name = strings.TrimSpace(r.URL.Query().Get("name"))

	if r.Method == http.MethodPost && r.Body != nil {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			if v, ok := body["source"].(string); ok && sourceID == "" {
				sourceID = v
			}
			if v, ok := body["url"].(string); ok && playlistURL == "" {
				playlistURL = v
			}
			if v, ok := body["epg"].(string); ok && epgURL == "" {
				epgURL = v
			}
			if v, ok := body["name"].(string); ok && name == "" {
				name = v
			}
		}
	}

	sourceID = xvynoraIPTVNormalizeSource(sourceID)
	return
}

func xvynoraIPTVNormalizeSource(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	switch id {
	case "", "all":
		return "all"
	case "pakistan":
		return "pk"
	default:
		return id
	}
}

func xvynoraIPTVSourceStatus() []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(IPTVOrgGetSources()))
	for _, source := range IPTVOrgGetSources() {
		result = append(result, map[string]interface{}{
			"id":          source.ID,
			"name":        source.Name,
			"type":        source.Type,
			"url":         source.URL,
			"category":    source.Category,
			"description": source.Description,
			"imported":    IPTVOrgProviderExists(source.URL),
		})
	}
	return result
}

func xvynoraIPTVChannels(force bool) []XVynoraIPTVChannelView {
	xvynoraIPTVSnapshotMu.RLock()
	if !force && time.Since(xvynoraIPTVSnap.builtAt) < 2*time.Second && xvynoraIPTVSnap.allCount == len(Data.Streams.All) {
		channels := xvynoraIPTVSnap.active
		xvynoraIPTVSnapshotMu.RUnlock()
		return channels
	}
	xvynoraIPTVSnapshotMu.RUnlock()

	xvynoraIPTVSnapshotMu.Lock()
	defer xvynoraIPTVSnapshotMu.Unlock()

	if !force && time.Since(xvynoraIPTVSnap.builtAt) < 2*time.Second && xvynoraIPTVSnap.allCount == len(Data.Streams.All) {
		return xvynoraIPTVSnap.active
	}

	base := Data.Streams.Active
	if len(base) == 0 {
		base = Data.Streams.All
	}

	channels := make([]XVynoraIPTVChannelView, 0, len(base))
	groupSet := make(map[string]struct{})

	if len(Data.XEPG.Channels) > 0 {
		xepg := make([]XEPGChannelStruct, 0, len(Data.XEPG.Channels))
		for _, value := range Data.XEPG.Channels {
			var channel XEPGChannelStruct
			if json.Unmarshal([]byte(mapToJSON(value)), &channel) == nil && !channel.XHideChannel {
				xepg = append(xepg, channel)
			}
		}
		sort.SliceStable(xepg, func(i, j int) bool {
			return xvynoraIPTVChannelSortKey(xepg[i]) < xvynoraIPTVChannelSortKey(xepg[j])
		})
		for _, channel := range xepg {
			view := xvynoraIPTVViewFromXEPG(channel)
			if view.ID == "" || view.Stream == "" {
				continue
			}
			channels = append(channels, view)
			if view.Group != "" {
				groupSet[view.Group] = struct{}{}
			}
		}
	} else {
		for index, value := range base {
			stream, ok := value.(map[string]string)
			if !ok {
				continue
			}
			view := xvynoraIPTVViewFromStream(stream, index)
			if view.ID == "" || view.Stream == "" {
				continue
			}
			channels = append(channels, view)
			if view.Group != "" {
				groupSet[view.Group] = struct{}{}
			}
		}
	}
	channels = xvynoraIPTVMergeChannels(channels)

	groups := make([]string, 0, len(groupSet))
	for group := range groupSet {
		groups = append(groups, group)
	}
	sort.Strings(groups)

	xvynoraIPTVSnap = xvynoraIPTVSnapshot{
		builtAt:  time.Now(),
		allCount: len(Data.Streams.All),
		active:   channels,
		groups:   groups,
	}

	return channels
}

func xvynoraIPTVInvalidateSnapshot() {
	xvynoraIPTVSnapshotMu.Lock()
	xvynoraIPTVSnap = xvynoraIPTVSnapshot{}
	xvynoraIPTVSnapshotMu.Unlock()
}

func xvynoraIPTVViewFromXEPG(channel XEPGChannelStruct) XVynoraIPTVChannelView {
	title := firstNonEmpty(channel.XName, channel.TvgName, channel.Name)
	group := firstNonEmpty(channel.XGroupTitle, channel.XCategory, channel.GroupTitle, "Live TV")
	source := xvynoraIPTVProviderSource(channel.FileM3UID, channel.FileM3UName)
	threadfinURL, _ := createStreamingURL("M3U", channel.FileM3UID, channel.XChannelID, title, channel.URL, channel.BackupChannel1, channel.BackupChannel2, channel.BackupChannel3)
	category := xvynoraIPTVCategorise(title, group, source)
	now, next := xvynoraIPTVNowNextFromXEPG(channel)

	view := XVynoraIPTVChannelView{
		ID:           firstNonEmpty(channel.XEPG, channel.ChannelUniqueID, channel.UUIDValue, xvynoraIPTVHash(channel.FileM3UID+"|"+channel.URL+"|"+title)),
		Title:        title,
		Name:         title,
		Stream:       channel.URL,
		ThreadfinURL: threadfinURL,
		Logo:         channel.TvgLogo,
		Group:        group,
		Category:     category,
		Country:      xvynoraIPTVCountry(source, group),
		Language:     xvynoraIPTVLanguage(group, title),
		Quality:      xvynoraIPTVQuality(title + " " + group + " " + channel.TvgID),
		Health:       "unknown",
		Status:       "unknown",
		Source:       source,
		Provider:     xvynoraIPTVProviderName(channel.FileM3UID, channel.FileM3UName, source),
		TvgID:        channel.TvgID,
		EPGID:        channel.XMapping,
		SkySports:    category == "Sky Sports",
		Description:  firstNonEmpty(channel.XDescription, xvynoraIPTVProgramTitle(now), group),
		Now:          now,
		Next:         next,
	}
	return xvynoraIPTVEnrichView(view)
}

func xvynoraIPTVViewFromStream(stream map[string]string, index int) XVynoraIPTVChannelView {
	title := firstNonEmpty(stream["name"], stream["tvg-name"], "Unknown Channel")
	group := firstNonEmpty(stream["group-title"], "Live TV")
	source := xvynoraIPTVProviderSource(stream["_file.m3u.id"], stream["_file.m3u.name"])
	category := xvynoraIPTVCategorise(title, group, source)
	id := firstNonEmpty(stream["_uuid.value"], stream["tvg-id"], stream["channelID"], fmt.Sprintf("stream-%d-%s", index, xvynoraIPTVHash(stream["url"])))

	view := XVynoraIPTVChannelView{
		ID:          id,
		Title:       title,
		Name:        title,
		Stream:      stream["url"],
		Logo:        stream["tvg-logo"],
		Group:       group,
		Category:    category,
		Country:     xvynoraIPTVCountry(source, group),
		Language:    xvynoraIPTVLanguage(group, title),
		Quality:     xvynoraIPTVQuality(title + " " + group),
		Health:      "unknown",
		Status:      "unknown",
		Source:      source,
		Provider:    xvynoraIPTVProviderName(stream["_file.m3u.id"], stream["_file.m3u.name"], source),
		TvgID:       stream["tvg-id"],
		EPGID:       stream["tvg-id"],
		SkySports:   category == "Sky Sports",
		Description: group,
	}
	return xvynoraIPTVEnrichView(view)
}

func xvynoraIPTVEnrichView(view XVynoraIPTVChannelView) XVynoraIPTVChannelView {
	entry, ok := xvynoraIPTVCatalogueFor(view.ID, view.TvgID, view.Title)
	if !ok {
		if xvynoraIPTVLogoURL(view.Logo) == "" {
			view.Logo = ""
		}
		if view.Stream != "" {
			view.StreamURLs = []string{view.Stream}
			view.Streams = []XVynoraIPTVStreamView{xvynoraIPTVStreamFromView(view)}
		}
		return view
	}

	view.ID = entry.ID
	view.AltNames = entry.AltNames
	view.Network = entry.Network
	view.Country = firstNonEmpty(entry.CountryName, entry.Country, view.Country)
	if len(entry.BroadcastArea) > 0 {
		view.Region = strings.Join(entry.BroadcastArea, ", ")
	}
	view.Language = firstNonEmpty(strings.Join(entry.LanguageNames, ", "), strings.Join(entry.Languages, ", "), view.Language)
	view.Category = firstNonEmpty(strings.Join(entry.CategoryNames, ", "), strings.Join(entry.Categories, ", "), view.Category)
	view.Group = firstNonEmpty(view.Group, entry.FeedName, "Live TV")
	view.TvgID = firstNonEmpty(view.TvgID, entry.EPGID)
	view.EPGID = firstNonEmpty(view.EPGID, entry.EPGID)
	view.EPGSources = entry.EPGSources
	view.LastChecked = entry.UpdatedAt
	view.Provider = firstNonEmpty(view.Provider, view.Source)
	view.Metadata = map[string]interface{}{
		"website":           entry.Website,
		"feed":              entry.Feed,
		"feed_name":         entry.FeedName,
		"owners":            entry.Owners,
		"broadcast_area":    entry.BroadcastArea,
		"timezone":          entry.Timezone,
		"format":            entry.Format,
		"logo_format":       entry.LogoFormat,
		"logo_width":        entry.LogoWidth,
		"logo_height":       entry.LogoHeight,
		"stream_quality":    entry.StreamQuality,
		"stream_title":      entry.StreamTitle,
		"stream_label":      entry.StreamLabel,
		"stream_referrer":   entry.StreamRef,
		"stream_user_agent": entry.StreamAgent,
	}
	if view.Logo == "" || xvynoraIPTVLogoURL(view.Logo) == "" {
		view.Logo = entry.Logo
	}
	if xvynoraIPTVLogoURL(view.Logo) == "" {
		view.Logo = ""
	}
	view.StreamURLs = append([]string{}, entry.StreamURLs...)
	if view.Stream != "" && !containsString(view.StreamURLs, view.Stream) {
		view.StreamURLs = append([]string{view.Stream}, view.StreamURLs...)
	}
	view.Streams = nil
	for _, url := range view.StreamURLs {
		stream := XVynoraIPTVStreamView{
			URL:      url,
			Source:   "iptv-org",
			Provider: entry.FeedName,
			Country:  entry.Country,
			Region:   view.Region,
			Language: strings.Join(entry.Languages, ", "),
			Health:   "unknown",
		}
		for _, catalogueStream := range entry.Streams {
			if catalogueStream.URL == url {
				stream.Feed = catalogueStream.Feed
				stream.Title = catalogueStream.Title
				stream.Availability = catalogueStream.Availability
				stream.Referrer = catalogueStream.Referrer
				stream.UserAgent = catalogueStream.UserAgent
				stream.Quality = catalogueStream.Quality
				break
			}
		}
		view.Streams = append(view.Streams, stream)
	}
	if view.Stream != "" {
		view.Streams = appendUniqueStreams([]XVynoraIPTVStreamView{xvynoraIPTVStreamFromView(view)}, view.Streams...)
	}
	return view
}

func xvynoraIPTVStreamFromView(view XVynoraIPTVChannelView) XVynoraIPTVStreamView {
	return XVynoraIPTVStreamView{
		URL:          view.Stream,
		Source:       view.Source,
		Provider:     view.Provider,
		Country:      view.Country,
		Region:       view.Region,
		Language:     view.Language,
		Quality:      view.Quality,
		Availability: "Unknown",
		Health:       view.Health,
	}
}

func xvynoraIPTVProviderName(providerID, providerName, source string) string {
	if source == "uk" || source == "pk" || source == "sports" {
		return "IPTV-org"
	}
	if providerID != "" {
		if provider, ok := Settings.Files.M3U[providerID].(map[string]interface{}); ok {
			if name, ok := provider["name"].(string); ok && strings.TrimSpace(name) != "" {
				return strings.TrimSpace(name)
			}
		}
	}
	return firstNonEmpty(providerName, "Unknown")
}

func xvynoraIPTVMergeKey(view XVynoraIPTVChannelView) string {
	if entry, ok := xvynoraIPTVCatalogueFor(view.ID, view.TvgID, view.Title); ok {
		return "id:" + entry.ID
	}
	if view.TvgID != "" {
		return "tvg:" + strings.ToLower(view.TvgID) + "|" + strings.ToLower(view.Country) + "|" + strings.ToLower(view.Region)
	}
	if view.Stream != "" {
		return "url:" + view.Stream
	}
	return "id:" + view.ID
}

func xvynoraIPTVMergeChannels(channels []XVynoraIPTVChannelView) []XVynoraIPTVChannelView {
	merged := make(map[string]int, len(channels))
	result := make([]XVynoraIPTVChannelView, 0, len(channels))
	for _, view := range channels {
		key := xvynoraIPTVMergeKey(view)
		if index, ok := merged[key]; ok {
			target := &result[index]
			if target.Stream == "" {
				target.Stream = view.Stream
			}
			if target.Logo == "" {
				target.Logo = view.Logo
			}
			if target.Provider == "" {
				target.Provider = view.Provider
			}
			if target.Source == "" {
				target.Source = view.Source
			}
			if target.Country == "" {
				target.Country = view.Country
			}
			target.StreamURLs = appendUnique(target.StreamURLs, view.StreamURLs...)
			target.Streams = appendUniqueStreams(target.Streams, view.Streams...)
			continue
		}
		merged[key] = len(result)
		result = append(result, view)
	}
	return result
}

func appendUnique(values []string, incoming ...string) []string {
	for _, value := range incoming {
		if value != "" && !containsString(values, value) {
			values = append(values, value)
		}
	}
	return values
}

func appendUniqueStreams(values []XVynoraIPTVStreamView, incoming ...XVynoraIPTVStreamView) []XVynoraIPTVStreamView {
	for _, value := range incoming {
		if value.URL == "" {
			continue
		}
		found := false
		for _, current := range values {
			if current.URL == value.URL {
				found = true
				break
			}
		}
		if !found {
			values = append(values, value)
		}
	}
	return values
}

func xvynoraIPTVFilterChannels(channels []XVynoraIPTVChannelView, r *http.Request) []XVynoraIPTVChannelView {
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
	source := xvynoraIPTVNormalizeSource(r.URL.Query().Get("source"))
	category := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("category")))
	country := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("country")))
	language := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("language")))
	quality := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("quality")))
	health := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("health")))
	provider := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
	region := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("region")))
	working := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("working")), "true")

	filtered := make([]XVynoraIPTVChannelView, 0, len(channels))
	for _, channel := range channels {
		if source != "" && source != "all" && channel.Source != source {
			continue
		}
		if provider != "" && !strings.Contains(strings.ToLower(channel.Provider), provider) {
			continue
		}
		if region != "" && !strings.Contains(strings.ToLower(channel.Region), region) {
			continue
		}
		haystack := strings.ToLower(strings.Join([]string{
			channel.Title,
			channel.Name,
			channel.ID,
			strings.Join(channel.AltNames, " "),
			channel.Network,
			channel.Group,
			channel.Category,
			channel.Country,
			channel.Language,
			channel.Quality,
			channel.TvgID,
		}, " "))
		if query != "" && !strings.Contains(haystack, query) {
			continue
		}
		if category != "" && !strings.Contains(strings.ToLower(channel.Category+" "+channel.Group), category) {
			continue
		}
		if country != "" && !strings.Contains(strings.ToLower(channel.Country), country) {
			continue
		}
		if language != "" && !strings.Contains(strings.ToLower(channel.Language), language) {
			continue
		}
		if quality != "" && strings.ToLower(channel.Quality) != quality {
			continue
		}
		if health != "" && strings.ToLower(channel.Health) != health && strings.ToLower(channel.Status) != health {
			continue
		}
		if working && channel.Status != "online" {
			continue
		}
		filtered = append(filtered, channel)
	}
	return filtered
}

func xvynoraIPTVPositiveInt(value string) int {
	var result int
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &result); err != nil || result < 0 {
		return 0
	}
	return result
}

func xvynoraIPTVDistinctValues(kind string) []string {
	cache, err := xvynoraIPTVCatalogueEntries()
	if err != nil {
		return []string{}
	}
	values := make(map[string]struct{})
	for _, channel := range cache.Channels {
		var items []string
		switch kind {
		case "categories":
			items = channel.Categories
		case "countries":
			items = []string{channel.Country}
		case "languages":
			items = channel.Languages
		}
		for _, value := range items {
			if value = strings.TrimSpace(value); value != "" {
				values[value] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func xvynoraIPTVFindChannel(id string) (XVynoraIPTVChannelView, bool) {
	id = strings.TrimSpace(id)
	for _, channel := range xvynoraIPTVChannels(false) {
		if channel.ID == id {
			return channel, true
		}
	}
	return XVynoraIPTVChannelView{}, false
}

func xvynoraIPTVProviderSource(providerID, providerName string) string {
	if providerID != "" {
		if provider, ok := Settings.Files.M3U[providerID].(map[string]interface{}); ok {
			if fileSource, ok := provider["file.source"].(string); ok {
				for _, source := range IPTVOrgGetSources() {
					if fileSource == source.URL {
						return source.ID
					}
				}
			}
		}
	}
	name := strings.ToLower(providerName)
	switch {
	case strings.Contains(name, "pakistan"):
		return "pk"
	case strings.Contains(name, "sports"):
		return "sports"
	case strings.Contains(name, "united kingdom"), strings.Contains(name, " uk"):
		return "uk"
	default:
		return "custom"
	}
}

func xvynoraIPTVCategorise(title, group, source string) string {
	text := strings.ToLower(title + " " + group)
	switch {
	case strings.Contains(text, "sky sports"), strings.Contains(text, "sky sport "):
		return "Sky Sports"
	case strings.Contains(text, "sport"):
		return "Sports"
	case strings.Contains(text, "news"):
		return "News"
	case source == "uk":
		return "UK"
	case source == "pk":
		return "Pakistan"
	case source == "sports":
		return "Sports"
	default:
		if group != "" {
			return group
		}
		return "Live TV"
	}
}

func xvynoraIPTVCountry(source, group string) string {
	switch source {
	case "uk":
		return "UK"
	case "pk":
		return "Pakistan"
	}
	group = strings.ToLower(group)
	switch {
	case strings.Contains(group, "pakistan"):
		return "Pakistan"
	case strings.Contains(group, "uk"), strings.Contains(group, "united kingdom"):
		return "UK"
	default:
		return ""
	}
}

func xvynoraIPTVLanguage(group, title string) string {
	text := strings.ToLower(group + " " + title)
	switch {
	case strings.Contains(text, "urdu"):
		return "Urdu"
	case strings.Contains(text, "punjabi"):
		return "Punjabi"
	case strings.Contains(text, "pashto"):
		return "Pashto"
	case strings.Contains(text, "english"), strings.Contains(text, "uk"):
		return "English"
	default:
		return ""
	}
}

func xvynoraIPTVQuality(text string) string {
	text = strings.ToLower(text)
	switch {
	case strings.Contains(text, "2160p"), strings.Contains(text, "uhd"), strings.Contains(text, "4k"):
		return "4K"
	case strings.Contains(text, "1080p"), strings.Contains(text, "fhd"):
		return "1080p"
	case strings.Contains(text, "720p"), strings.Contains(text, " hd"):
		return "720p"
	case strings.Contains(text, "480p"), strings.Contains(text, " sd"):
		return "480p"
	default:
		return ""
	}
}

func xvynoraIPTVNowNext(channel XVynoraIPTVChannelView) (*XVynoraIPTVProgram, *XVynoraIPTVProgram) {
	for _, value := range Data.XEPG.Channels {
		var xepg XEPGChannelStruct
		if json.Unmarshal([]byte(mapToJSON(value)), &xepg) == nil && (xepg.XEPG == channel.ID || xepg.ChannelUniqueID == channel.ID || xepg.TvgID == channel.TvgID) {
			return xvynoraIPTVNowNextFromXEPG(xepg)
		}
	}
	return nil, nil
}

func xvynoraIPTVNowNextFromXEPG(channel XEPGChannelStruct) (*XVynoraIPTVProgram, *XVynoraIPTVProgram) {
	if channel.XmltvFile == "" || channel.XMapping == "" || strings.Contains(channel.XmltvFile, "Threadfin Dummy") {
		return nil, nil
	}

	var xmltv XMLTV
	if err := getLocalXMLTV(System.Folder.Data+channel.XmltvFile, &xmltv); err != nil {
		return nil, nil
	}

	now := time.Now()
	var current *XVynoraIPTVProgram
	var next *XVynoraIPTVProgram
	for _, program := range xmltv.Program {
		if program == nil || program.Channel != channel.XMapping {
			continue
		}
		start, startOK := xvynoraParseXMLTVTime(program.Start)
		stop, stopOK := xvynoraParseXMLTVTime(program.Stop)
		if !startOK || !stopOK {
			continue
		}
		view := xvynoraIPTVProgramFromXML(program, start, stop, now)
		if (now.Equal(start) || now.After(start)) && now.Before(stop) {
			current = &view
			continue
		}
		if start.After(now) && (next == nil || start.Before(xvynoraMustTime(next.Start))) {
			next = &view
		}
	}
	return current, next
}

func xvynoraIPTVProgramFromXML(program *Program, start, stop, now time.Time) XVynoraIPTVProgram {
	title := "Unavailable"
	if len(program.Title) > 0 && program.Title[0] != nil && strings.TrimSpace(program.Title[0].Value) != "" {
		title = strings.TrimSpace(program.Title[0].Value)
	}
	desc := ""
	if len(program.Desc) > 0 && program.Desc[0] != nil {
		desc = strings.TrimSpace(program.Desc[0].Value)
	}
	progress := 0.0
	if stop.After(start) && now.After(start) && now.Before(stop) {
		progress = now.Sub(start).Seconds() / stop.Sub(start).Seconds()
	}
	return XVynoraIPTVProgram{
		Title:       title,
		Description: desc,
		Start:       start.Format(time.RFC3339),
		Stop:        stop.Format(time.RFC3339),
		Progress:    progress,
		Source:      "Threadfin/XEPG",
	}
}

func xvynoraParseXMLTVTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"20060102150405 -0700",
		"20060102150405 -0700 MST",
		"20060102150405",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t, true
		}
	}
	if len(value) >= 14 {
		if t, err := time.Parse("20060102150405", value[:14]); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func xvynoraMustTime(value string) time.Time {
	t, _ := time.Parse(time.RFC3339, value)
	return t
}

func xvynoraIPTVProgramTitle(program *XVynoraIPTVProgram) string {
	if program == nil {
		return ""
	}
	return program.Title
}

func xvynoraIPTVProbeURL(streamURL string) (string, error) {
	streamURL = strings.TrimSpace(streamURL)
	if streamURL == "" {
		return "unknown", errors.New("stream URL unavailable")
	}

	client := &http.Client{Timeout: 6 * time.Second}
	req, err := http.NewRequest(http.MethodHead, streamURL, nil)
	if err != nil {
		return "unknown", err
	}
	req.Header.Set("User-Agent", "XVynora-IPTV/1.0")

	resp, err := client.Do(req)
	if err != nil {
		req, getErr := http.NewRequest(http.MethodGet, streamURL, nil)
		if getErr != nil {
			return "unknown", getErr
		}
		req.Header.Set("User-Agent", "XVynora-IPTV/1.0")
		req.Header.Set("Range", "bytes=0-0")
		resp, err = client.Do(req)
		if err != nil {
			return "failed", err
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return "unknown", errors.New("stream reachable; playback not verified")
	}
	return "failed", fmt.Errorf("stream returned HTTP %d", resp.StatusCode)
}

func xvynoraIPTVImportSource(sourceID, playlistURL, epgURL, name string) (interface{}, error) {
	if playlistURL != "" || epgURL != "" {
		return xvynoraIPTVImportCustom(playlistURL, epgURL, name)
	}

	if sourceID == "" || sourceID == "all" {
		imported := make([]interface{}, 0, len(IPTVOrgGetSources()))
		for _, source := range IPTVOrgGetSources() {
			if IPTVOrgProviderExists(source.URL) {
				continue
			}
			provider, err := IPTVOrgAddProvider(source.ID)
			if err != nil {
				return imported, err
			}
			imported = append(imported, provider)
		}
		if err := xvynoraIPTVRefreshAll(); err != nil {
			return imported, err
		}
		if err := xvynoraIPTVRefreshCatalogue(); err != nil {
			showInfo("IPTV-org catalogue refresh failed: " + err.Error())
		}
		return imported, nil
	}

	provider, err := IPTVOrgAddProvider(sourceID)
	if err != nil {
		return nil, err
	}
	providerID, _ := provider["id.provider"].(string)
	if err := getProviderData("m3u", providerID); err != nil {
		return nil, err
	}
	if err := buildDatabaseDVR(); err != nil {
		return nil, err
	}
	buildXEPG(false)
	return provider, nil
}

func xvynoraIPTVImportCustom(playlistURL, epgURL, name string) (interface{}, error) {
	result := make(map[string]interface{})
	if playlistURL != "" {
		provider, err := xvynoraIPTVAddProvider("m3u", playlistURL, firstNonEmpty(name, "XVynora M3U"))
		if err != nil {
			return result, err
		}
		result["m3u"] = provider
	}
	if epgURL != "" {
		provider, err := xvynoraIPTVAddProvider("xmltv", epgURL, firstNonEmpty(name, "XVynora XMLTV"))
		if err != nil {
			return result, err
		}
		result["xmltv"] = provider
	}
	if len(result) == 0 {
		return nil, errors.New("missing source URL")
	}
	if err := xvynoraIPTVRefreshAll(); err != nil {
		return result, err
	}
	return result, nil
}

func xvynoraIPTVAddProvider(fileType, sourceURL, name string) (map[string]interface{}, error) {
	if sourceURL == "" {
		return nil, errors.New("missing provider URL")
	}
	indicator := "M"
	extension := ".m3u"
	files := Settings.Files.M3U
	if fileType == "xmltv" {
		indicator = "X"
		extension = ".xml"
		files = Settings.Files.XMLTV
	}
	if files == nil {
		files = make(map[string]interface{})
	}
	for _, value := range files {
		if provider, ok := value.(map[string]interface{}); ok && provider["file.source"] == sourceURL {
			return nil, fmt.Errorf("%s source already imported", strings.ToUpper(fileType))
		}
	}
	providerID := indicator + randomString(19)
	provider := map[string]interface{}{
		"id.provider":           providerID,
		"name":                  name,
		"description":           "XVynora TV source",
		"type":                  fileType,
		"file.source":           sourceURL,
		"file.Threadfin":        providerID + extension,
		"buffer":                "-",
		"refresh":               "24",
		"tuner":                 Settings.Tuner,
		"counter.error":         0,
		"counter.download":      0,
		"last.update":           "",
		"compatibility":         map[string]interface{}{},
		"provider.availability": true,
	}
	files[providerID] = provider
	if fileType == "xmltv" {
		Settings.Files.XMLTV = files
	} else {
		Settings.Files.M3U = files
	}
	if err := saveSettings(Settings); err != nil {
		delete(files, providerID)
		return nil, err
	}
	if err := getProviderData(fileType, providerID); err != nil {
		delete(files, providerID)
		_ = saveSettings(Settings)
		return nil, err
	}
	return provider, nil
}

func xvynoraIPTVUpdateSource(sourceID string) error {
	if sourceID == "" || sourceID == "all" {
		return xvynoraIPTVRefreshAll()
	}
	source, ok := IPTVOrgGetSource(sourceID)
	if !ok {
		return fmt.Errorf("unknown IPTV-org source: %s", sourceID)
	}
	for id, value := range Settings.Files.M3U {
		provider, ok := value.(map[string]interface{})
		if ok && provider["file.source"] == source.URL {
			if err := getProviderData("m3u", id); err != nil {
				return err
			}
			if err := buildDatabaseDVR(); err != nil {
				return err
			}
			buildXEPG(false)
			return nil
		}
	}
	return fmt.Errorf("source is not imported: %s", source.Name)
}

func xvynoraIPTVRemoveSource(sourceID string) error {
	if sourceID == "" || sourceID == "all" {
		for _, source := range IPTVOrgGetSources() {
			_ = IPTVOrgRemoveProvider(source.ID)
		}
		if err := buildDatabaseDVR(); err != nil {
			return err
		}
		buildXEPG(false)
		return nil
	}
	if err := IPTVOrgRemoveProvider(sourceID); err != nil {
		return err
	}
	if err := buildDatabaseDVR(); err != nil {
		return err
	}
	buildXEPG(false)
	return nil
}

func xvynoraIPTVRefreshAll() error {
	if err := getProviderData("m3u", ""); err != nil {
		return err
	}
	if Settings.EpgSource == "XEPG" {
		if err := getProviderData("xmltv", ""); err != nil {
			return err
		}
	}
	if err := buildDatabaseDVR(); err != nil {
		return err
	}
	buildXEPG(false)
	return nil
}

func xvynoraIPTVChannelSortKey(channel XEPGChannelStruct) string {
	if channel.TvgChno != "" {
		return fmt.Sprintf("%012s-%s", channel.TvgChno, channel.XName)
	}
	return strings.ToLower(firstNonEmpty(channel.XName, channel.Name))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func xvynoraIPTVHash(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}
