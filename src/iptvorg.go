package src

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const iptvOrgBaseURL = "https://iptv-org.github.io/iptv"

type IPTVOrgSource struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

var IPTVOrgSources = []IPTVOrgSource{
	{
		ID:          "uk",
		Name:        "🇬🇧 United Kingdom",
		Type:        "country",
		URL:         iptvOrgBaseURL + "/countries/uk.m3u",
		Description: "Publicly available UK channels",
	},
	{
		ID:          "pk",
		Name:        "🇵🇰 Pakistan",
		Type:        "country",
		URL:         iptvOrgBaseURL + "/countries/pk.m3u",
		Description: "Publicly available Pakistani channels",
	},
	{
		ID:          "sports",
		Name:        "⚽ Sports",
		Type:        "category",
		URL:         iptvOrgBaseURL + "/categories/sports.m3u",
		Description: "Publicly available sports channels",
	},
}

func IPTVOrgGetSources() []IPTVOrgSource {
	return IPTVOrgSources
}

func IPTVOrgGetSource(id string) (IPTVOrgSource, bool) {
	for _, source := range IPTVOrgSources {
		if source.ID == id {
			return source, true
		}
	}

	return IPTVOrgSource{}, false
}

func IPTVOrgDownload(id string) ([]byte, error) {
	source, ok := IPTVOrgGetSource(id)
	if !ok {
		return nil, fmt.Errorf("unknown IPTV-org source: %s", id)
	}

	client := &http.Client{}

	req, err := http.NewRequest(http.MethodGet, source.URL, nil)
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

func IPTVOrgSourceJSON() ([]byte, error) {
	return json.Marshal(IPTVOrgSources)
}

func IPTVOrgMatchesChannel(name string, keywords ...string) bool {
	name = strings.ToLower(name)

	for _, keyword := range keywords {
		if strings.Contains(name, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}
