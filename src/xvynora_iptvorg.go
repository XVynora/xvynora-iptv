package src

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const xvynoraIPTVOrgBase = "https://iptv-org.github.io/iptv"

type XVynoraIPTVSource struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Country     string `json:"country"`
	Category    string `json:"category"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

type XVynoraIPTVChannel struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Country     string   `json:"country,omitempty"`
	Logo        string   `json:"logo,omitempty"`
	Group       string   `json:"group"`
	Stream      string   `json:"stream,omitempty"`
	EPGID       string   `json:"epg_id,omitempty"`
	Language    []string `json:"language,omitempty"`
	IsSkySports bool     `json:"sky_sports,omitempty"`
}

var xvynoraIPTVSources = []XVynoraIPTVSource{
	{
		ID:          "uk",
		Name:        "United Kingdom",
		Country:     "GB",
		Category:    "country",
		Description: "UK television channels from IPTV-org",
		URL:         xvynoraIPTVOrgBase + "/countries/uk.m3u",
	},
	{
		ID:          "pakistan",
		Name:        "Pakistan",
		Country:     "PK",
		Category:    "country",
		Description: "Pakistani television channels from IPTV-org",
		URL:         xvynoraIPTVOrgBase + "/countries/pk.m3u",
	},
	{
		ID:          "sports",
		Name:        "Sports",
		Category:    "sports",
		Description: "Sports channels from IPTV-org",
		URL:         xvynoraIPTVOrgBase + "/categories/sports.m3u",
	},
}

func xvynoraIPTVGetSource(id string) (XVynoraIPTVSource, bool) {
	for _, s := range xvynoraIPTVSources {
		if s.ID == id {
			return s, true
		}
	}
	return XVynoraIPTVSource{}, false
}

func xvynoraIPTVDownload(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}

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
		return nil, fmt.Errorf("IPTV-org returned HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func xvynoraIPTVParseM3U(data []byte) []XVynoraIPTVChannel {
	lines := strings.Split(string(data), "\n")
	channels := make([]XVynoraIPTVChannel, 0)

	var current XVynoraIPTVChannel

	for _, raw := range lines {
		line := strings.TrimSpace(raw)

		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#EXTINF:") {
			current = XVynoraIPTVChannel{}

			if i := strings.Index(line, ","); i >= 0 {
				current.Name = strings.TrimSpace(line[i+1:])
			}

			attrs := line[:strings.Index(line, ",")]

			for _, field := range strings.Fields(attrs) {
				switch {
				case strings.HasPrefix(field, `group-title="`):
					current.Group = strings.Trim(field[len(`group-title="`):], `"`)
				case strings.HasPrefix(field, `tvg-id="`):
					current.EPGID = strings.Trim(field[len(`tvg-id="`):], `"`)
				case strings.HasPrefix(field, `tvg-logo="`):
					current.Logo = strings.Trim(field[len(`tvg-logo="`):], `"`)
				}
			}

			current.Category = current.Group

			lower := strings.ToLower(current.Name)

			if strings.Contains(lower, "sky sports") ||
				strings.Contains(lower, "sky sports news") ||
				strings.Contains(lower, "sky sports main event") ||
				strings.Contains(lower, "sky sports mix") {
				current.IsSkySports = true
				current.Category = "Sky Sports"
			}

			if current.Category == "" {
				current.Category = "Other"
			}

			continue
		}

		if strings.HasPrefix(line, "#") {
			continue
		}

		if current.Name != "" {
			current.Stream = line
			channels = append(channels, current)
			current = XVynoraIPTVChannel{}
		}
	}

	return channels
}

func xvynoraIPTVJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
