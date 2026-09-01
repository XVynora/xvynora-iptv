package src

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const xvynoraIPTVCatalogueCacheFile = "xvynora-iptv-catalogue.json"

type XVynoraIPTVCatalogueEntry struct {
	ID            string                       `json:"id"`
	Name          string                       `json:"name"`
	AltNames      []string                     `json:"alt_names,omitempty"`
	Network       string                       `json:"network,omitempty"`
	Owners        []string                     `json:"owners,omitempty"`
	Country       string                       `json:"country,omitempty"`
	Categories    []string                     `json:"categories,omitempty"`
	Website       string                       `json:"website,omitempty"`
	Feed          string                       `json:"feed,omitempty"`
	FeedName      string                       `json:"feed_name,omitempty"`
	BroadcastArea []string                     `json:"broadcast_area,omitempty"`
	Languages     []string                     `json:"languages,omitempty"`
	Timezone      string                       `json:"timezone,omitempty"`
	Format        string                       `json:"format,omitempty"`
	Logo          string                       `json:"logo,omitempty"`
	LogoFormat    string                       `json:"logo_format,omitempty"`
	LogoWidth     int                          `json:"logo_width,omitempty"`
	LogoHeight    int                          `json:"logo_height,omitempty"`
	StreamURLs    []string                     `json:"stream_urls,omitempty"`
	StreamQuality []string                     `json:"stream_quality,omitempty"`
	StreamTitle   []string                     `json:"stream_title,omitempty"`
	StreamLabel   []string                     `json:"stream_label,omitempty"`
	StreamRef     []string                     `json:"stream_referrer,omitempty"`
	StreamAgent   []string                     `json:"stream_user_agent,omitempty"`
	EPGID         string                       `json:"epg_id,omitempty"`
	EPGSources    []string                     `json:"epg_sources,omitempty"`
	CategoryNames []string                     `json:"category_names,omitempty"`
	CountryName   string                       `json:"country_name,omitempty"`
	LanguageNames []string                     `json:"language_names,omitempty"`
	Feeds         []XVynoraIPTVCatalogueFeed   `json:"feeds,omitempty"`
	Streams       []XVynoraIPTVCatalogueStream `json:"streams,omitempty"`
	UpdatedAt     string                       `json:"last_checked,omitempty"`
}

type XVynoraIPTVCatalogueFeed struct {
	ID            string   `json:"id"`
	Name          string   `json:"name,omitempty"`
	BroadcastArea []string `json:"broadcast_area,omitempty"`
	Languages     []string `json:"languages,omitempty"`
	Timezones     []string `json:"timezones,omitempty"`
	Format        string   `json:"format,omitempty"`
	IsMain        bool     `json:"is_main,omitempty"`
}

type XVynoraIPTVCatalogueStream struct {
	URL          string `json:"url"`
	Feed         string `json:"feed,omitempty"`
	Title        string `json:"title,omitempty"`
	Label        string `json:"label,omitempty"`
	Quality      string `json:"quality,omitempty"`
	Referrer     string `json:"referrer,omitempty"`
	UserAgent    string `json:"user_agent,omitempty"`
	Availability string `json:"availability,omitempty"`
}

type xvynoraIPTVCatalogueCache struct {
	LastRefresh string                      `json:"last_refresh"`
	Channels    []XVynoraIPTVCatalogueEntry `json:"channels"`
	Categories  []map[string]interface{}    `json:"categories,omitempty"`
	Countries   []map[string]interface{}    `json:"countries,omitempty"`
	Languages   []map[string]interface{}    `json:"languages,omitempty"`
}

var (
	xvynoraIPTVCatalogueMu     sync.RWMutex
	xvynoraIPTVCatalogue       xvynoraIPTVCatalogueCache
	xvynoraIPTVCatalogueLoaded bool
)

func xvynoraIPTVCataloguePath() string {
	return filepath.Join(System.Folder.Data, xvynoraIPTVCatalogueCacheFile)
}

func xvynoraIPTVLoadCatalogue() {
	xvynoraIPTVCatalogueMu.Lock()
	defer xvynoraIPTVCatalogueMu.Unlock()
	if xvynoraIPTVCatalogueLoaded {
		return
	}
	xvynoraIPTVCatalogueLoaded = true
	data, err := os.ReadFile(xvynoraIPTVCataloguePath())
	if err == nil {
		_ = json.Unmarshal(data, &xvynoraIPTVCatalogue)
	}
}

func xvynoraIPTVRefreshCatalogue() error {
	channels, err := xvynoraIPTVDownloadJSON("https://iptv-org.github.io/api/channels.json")
	if err != nil {
		return err
	}
	feeds, err := xvynoraIPTVDownloadJSON("https://iptv-org.github.io/api/feeds.json")
	if err != nil {
		return err
	}
	streams, err := xvynoraIPTVDownloadJSON("https://iptv-org.github.io/api/streams.json")
	if err != nil {
		return err
	}
	logos, err := xvynoraIPTVDownloadJSON("https://iptv-org.github.io/api/logos.json")
	if err != nil {
		return err
	}
	guides, err := xvynoraIPTVDownloadJSON("https://iptv-org.github.io/api/guides.json")
	if err != nil {
		return err
	}
	categories, err := xvynoraIPTVDownloadJSON("https://iptv-org.github.io/api/categories.json")
	if err != nil {
		return err
	}
	countries, err := xvynoraIPTVDownloadJSON("https://iptv-org.github.io/api/countries.json")
	if err != nil {
		return err
	}
	languages, err := xvynoraIPTVDownloadJSON("https://iptv-org.github.io/api/languages.json")
	if err != nil {
		return err
	}

	cache := xvynoraIPTVCatalogueCache{
		LastRefresh: time.Now().UTC().Format(time.RFC3339),
		Channels:    xvynoraIPTVBuildCatalogue(channels, feeds, streams, logos, guides, categories, countries, languages),
		Categories:  categories,
		Countries:   countries,
		Languages:   languages,
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(System.Folder.Data, 0755); err != nil {
		return err
	}
	tmp := xvynoraIPTVCataloguePath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, xvynoraIPTVCataloguePath()); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	xvynoraIPTVCatalogueMu.Lock()
	xvynoraIPTVCatalogue = cache
	xvynoraIPTVCatalogueLoaded = true
	xvynoraIPTVCatalogueMu.Unlock()
	xvynoraIPTVInvalidateSnapshot()
	return nil
}

func xvynoraIPTVDownloadJSON(url string) ([]map[string]interface{}, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "XVynora-IPTV/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IPTV-org returned HTTP %d for %s", resp.StatusCode, url)
	}
	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func xvynoraIPTVString(value interface{}) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func xvynoraIPTVStrings(value interface{}) []string {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if value := xvynoraIPTVString(item); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func xvynoraIPTVInt(value interface{}) int {
	var result int
	_, _ = fmt.Sscanf(xvynoraIPTVString(value), "%d", &result)
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func xvynoraIPTVBuildCatalogue(channels, feeds, streams, logos, guides, categories, countries, languages []map[string]interface{}) []XVynoraIPTVCatalogueEntry {
	feedByChannel := make(map[string][]map[string]interface{})
	for _, feed := range feeds {
		if channel := xvynoraIPTVString(feed["channel"]); channel != "" {
			feedByChannel[channel] = append(feedByChannel[channel], feed)
		}
	}
	logoByChannel := make(map[string]map[string]interface{})
	logoByFeed := make(map[string]map[string]interface{})
	for _, logo := range logos {
		id := xvynoraIPTVString(logo["channel"])
		if id != "" && (logo["in_use"] == true || logoByChannel[id] == nil) {
			if feed := xvynoraIPTVString(logo["feed"]); feed != "" {
				key := id + "|" + feed
				if logo["in_use"] == true || logoByFeed[key] == nil {
					logoByFeed[key] = logo
				}
			} else {
				logoByChannel[id] = logo
			}
		}
	}
	streamByChannel := make(map[string][]map[string]interface{})
	for _, stream := range streams {
		if id := xvynoraIPTVString(stream["channel"]); id != "" {
			streamByChannel[id] = append(streamByChannel[id], stream)
		}
	}
	guideByChannel := make(map[string][]string)
	for _, guide := range guides {
		id := xvynoraIPTVString(guide["channel"])
		if id != "" {
			guideByChannel[id] = append(guideByChannel[id], xvynoraIPTVStrings(guide["sources"])...)
		}
	}
	categoryNames := make(map[string]string)
	for _, item := range categories {
		categoryNames[xvynoraIPTVString(item["id"])] = xvynoraIPTVString(item["name"])
	}
	countryNames := make(map[string]string)
	for _, item := range countries {
		countryNames[xvynoraIPTVString(item["code"])] = xvynoraIPTVString(item["name"])
	}
	languageNames := make(map[string]string)
	for _, item := range languages {
		languageNames[xvynoraIPTVString(item["code"])] = xvynoraIPTVString(item["name"])
	}

	result := make([]XVynoraIPTVCatalogueEntry, 0, len(channels))
	for _, channel := range channels {
		id := xvynoraIPTVString(channel["id"])
		if id == "" {
			continue
		}
		entry := XVynoraIPTVCatalogueEntry{
			ID:            id,
			Name:          xvynoraIPTVString(channel["name"]),
			AltNames:      xvynoraIPTVStrings(channel["alt_names"]),
			Network:       xvynoraIPTVString(channel["network"]),
			Owners:        xvynoraIPTVStrings(channel["owners"]),
			Country:       xvynoraIPTVString(channel["country"]),
			CountryName:   countryNames[xvynoraIPTVString(channel["country"])],
			Categories:    xvynoraIPTVStrings(channel["categories"]),
			Website:       xvynoraIPTVString(channel["website"]),
			BroadcastArea: xvynoraIPTVStrings(channel["broadcast_area"]),
			Languages:     xvynoraIPTVStrings(channel["languages"]),
			Timezone:      xvynoraIPTVString(channel["timezone"]),
			Format:        xvynoraIPTVString(channel["format"]),
			EPGID:         firstNonEmpty(xvynoraIPTVString(channel["epg_id"]), id),
			EPGSources:    guideByChannel[id],
			UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
		}
		for _, category := range entry.Categories {
			if name := categoryNames[category]; name != "" {
				entry.CategoryNames = append(entry.CategoryNames, name)
			}
		}
		for _, language := range entry.Languages {
			if name := languageNames[language]; name != "" {
				entry.LanguageNames = append(entry.LanguageNames, name)
			}
		}
		for _, feed := range feedByChannel[id] {
			feedView := XVynoraIPTVCatalogueFeed{
				ID:            xvynoraIPTVString(feed["id"]),
				Name:          xvynoraIPTVString(feed["name"]),
				BroadcastArea: xvynoraIPTVStrings(feed["broadcast_area"]),
				Languages:     xvynoraIPTVStrings(feed["languages"]),
				Timezones:     xvynoraIPTVStrings(feed["timezones"]),
				Format:        xvynoraIPTVString(feed["format"]),
				IsMain:        feed["is_main"] == true,
			}
			entry.Feeds = append(entry.Feeds, feedView)
			if logo := logoByFeed[id+"|"+feedView.ID]; logo != nil && entry.Logo == "" {
				entry.Logo = xvynoraIPTVLogoURL(logo["url"])
			}
			if feedView.IsMain || entry.Feed == "" {
				entry.Feed = feedView.ID
				entry.FeedName = firstNonEmpty(feedView.Name, feedView.ID)
				entry.BroadcastArea = appendUniqueStrings(entry.BroadcastArea, feedView.BroadcastArea...)
				entry.Languages = appendUniqueStrings(entry.Languages, feedView.Languages...)
				entry.Timezone = firstNonEmpty(entry.Timezone, firstNonEmpty(feedView.Timezones...))
				entry.Format = firstNonEmpty(entry.Format, feedView.Format)
			}
		}
		for _, stream := range streamByChannel[id] {
			url := xvynoraIPTVString(stream["url"])
			if url == "" || containsString(entry.StreamURLs, url) {
				continue
			}
			entry.StreamURLs = append(entry.StreamURLs, url)
			entry.StreamQuality = append(entry.StreamQuality, xvynoraIPTVString(stream["quality"]))
			entry.StreamTitle = append(entry.StreamTitle, xvynoraIPTVString(stream["title"]))
			entry.StreamLabel = append(entry.StreamLabel, xvynoraIPTVString(stream["label"]))
			entry.StreamRef = append(entry.StreamRef, xvynoraIPTVString(stream["referrer"]))
			entry.StreamAgent = append(entry.StreamAgent, xvynoraIPTVString(stream["user_agent"]))
			entry.Streams = append(entry.Streams, XVynoraIPTVCatalogueStream{
				URL:          url,
				Feed:         xvynoraIPTVString(stream["feed"]),
				Title:        xvynoraIPTVString(stream["title"]),
				Label:        xvynoraIPTVString(stream["label"]),
				Quality:      xvynoraIPTVString(stream["quality"]),
				Referrer:     xvynoraIPTVString(stream["referrer"]),
				UserAgent:    xvynoraIPTVString(stream["user_agent"]),
				Availability: firstNonEmpty(xvynoraIPTVString(stream["label"]), "Unknown"),
			})
		}
		if logo := logoByChannel[id]; logo != nil {
			entry.Logo = firstNonEmpty(entry.Logo, xvynoraIPTVLogoURL(logo["url"]))
			entry.LogoFormat = xvynoraIPTVString(logo["format"])
			entry.LogoWidth = xvynoraIPTVInt(logo["width"])
			entry.LogoHeight = xvynoraIPTVInt(logo["height"])
		}
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func xvynoraIPTVLogoURL(value interface{}) string {
	parsed, err := url.Parse(strings.TrimSpace(xvynoraIPTVString(value)))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if strings.HasSuffix(host, "."+"imgur"+".com") || host == "imgur.com" {
		return ""
	}
	return parsed.String()
}

func appendUniqueStrings(values []string, incoming ...string) []string {
	for _, value := range incoming {
		if value != "" && !containsString(values, value) {
			values = append(values, value)
		}
	}
	return values
}

func xvynoraIPTVCatalogueEntries() (xvynoraIPTVCatalogueCache, error) {
	xvynoraIPTVLoadCatalogue()
	xvynoraIPTVCatalogueMu.RLock()
	cache := xvynoraIPTVCatalogue
	xvynoraIPTVCatalogueMu.RUnlock()
	if len(cache.Channels) == 0 {
		return cache, errors.New("IPTV-org catalogue is unavailable")
	}
	return cache, nil
}

func xvynoraIPTVCatalogueFor(id, tvgID, name string) (XVynoraIPTVCatalogueEntry, bool) {
	cache, err := xvynoraIPTVCatalogueEntries()
	if err != nil {
		return XVynoraIPTVCatalogueEntry{}, false
	}
	for _, entry := range cache.Channels {
		if entry.ID == id || entry.ID == tvgID {
			return entry, true
		}
	}
	normalized := strings.ToLower(strings.TrimSpace(name))
	for _, entry := range cache.Channels {
		if normalized != "" && strings.ToLower(strings.TrimSpace(entry.Name)) == normalized {
			return entry, true
		}
		for _, alt := range entry.AltNames {
			if normalized != "" && strings.ToLower(strings.TrimSpace(alt)) == normalized {
				return entry, true
			}
		}
	}
	return XVynoraIPTVCatalogueEntry{}, false
}
