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

// IPTVOrgImportSource adds an IPTV-ORG playlist to Threadfin's normal M3U
// provider system, then lets the existing provider/database pipeline process it.
func IPTVOrgImportSource(id string) error {
	source, ok := IPTVOrgGetSource(id)
	if !ok {
		return fmt.Errorf("unknown IPTV-org source: %s", id)
	}

	for _, value := range Settings.Files.M3U {
		if data, ok := value.(map[string]interface{}); ok {
			if data["file.source"] == source.URL {
				return fmt.Errorf("source already imported: %s", source.Name)
			}
		}
	}

	providerID := "M" + randomString(19)

	provider := map[string]interface{}{
		"id.provider":          providerID,
		"name":                 source.Name,
		"description":          source.Description,
		"type":                 "m3u",
		"file.Threadfin":       providerID + ".m3u",
		"file.source":          source.URL,
		"tuner":                1.0,
		"http_proxy.ip":        "",
		"http_proxy.port":      "",
		"compatibility":        map[string]interface{}{},
		"counter.error":        0.0,
		"counter.download":     0.0,
		"provider.availability": 100,
	}

	if Settings.Files.M3U == nil {
		Settings.Files.M3U = make(map[string]interface{})
	}

	Settings.Files.M3U[providerID] = provider

	if err := saveSettings(Settings); err != nil {
		return err
	}

	if err := getProviderData("m3u", providerID); err != nil {
		delete(Settings.Files.M3U, providerID)
		_ = saveSettings(Settings)
		return err
	}

	if err := buildDatabaseDVR(); err != nil {
		return err
	}

	buildXEPG(false)

	return nil
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
