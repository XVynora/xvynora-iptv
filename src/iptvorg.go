package src

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const IPTVOrgBaseURL = "https://iptv-org.github.io/iptv"

type IPTVOrgSource struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

var IPTVOrgSources = []IPTVOrgSource{
	{
		ID:          "uk",
		Name:        "United Kingdom",
		Type:        "country",
		URL:         IPTVOrgBaseURL + "/countries/uk.m3u",
		Description: "IPTV-org United Kingdom channels",
		Category:    "United Kingdom",
	},
	{
		ID:          "pk",
		Name:        "Pakistan",
		Type:        "country",
		URL:         IPTVOrgBaseURL + "/countries/pk.m3u",
		Description: "IPTV-org Pakistan channels",
		Category:    "Pakistan",
	},
	{
		ID:          "sports",
		Name:        "Sports",
		Type:        "category",
		URL:         IPTVOrgBaseURL + "/categories/sports.m3u",
		Description: "IPTV-org sports channels",
		Category:    "Sports",
	},
}

func IPTVOrgGetSources() []IPTVOrgSource {
	return IPTVOrgSources
}

func IPTVOrgGetSource(id string) (IPTVOrgSource, bool) {
	id = strings.ToLower(strings.TrimSpace(id))

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

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

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

func IPTVOrgCategoriseChannel(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))

	switch {
	case strings.Contains(name, "sky sports"):
		return "Sky Sports"
	case strings.Contains(name, "sky sport"):
		return "Sky Sports"
	case strings.Contains(name, "sport"):
		return "Sports"
	case strings.Contains(name, "bbc"):
		return "BBC"
	case strings.Contains(name, "itv"):
		return "ITV"
	case strings.Contains(name, "channel 4"),
		strings.Contains(name, "channel4"):
		return "Channel 4"
	case strings.Contains(name, "channel 5"),
		strings.Contains(name, "channel5"):
		return "Channel 5"
	default:
		return ""
	}
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

func IPTVOrgProvider(sourceID string) (map[string]interface{}, error) {
	source, ok := IPTVOrgGetSource(sourceID)
	if !ok {
		return nil, fmt.Errorf("unknown IPTV-org source: %s", sourceID)
	}

	providerID := "M" + randomString(19)

	provider := make(map[string]interface{})
	provider["id.provider"] = providerID
	provider["name"] = "XVynora · " + source.Name
	provider["description"] = source.Description
	provider["type"] = "m3u"
	provider["file.source"] = source.URL
	provider["file.Threadfin"] = providerID + ".m3u"
	provider["buffer"] = "-"
	provider["refresh"] = "24"

	return provider, nil
}

func IPTVOrgProviderExists(sourceURL string) bool {
	for _, value := range Settings.Files.M3U {
		provider, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		if provider["file.source"] == sourceURL {
			return true
		}
	}

	return false
}

func IPTVOrgAddProvider(sourceID string) (map[string]interface{}, error) {
	source, ok := IPTVOrgGetSource(sourceID)
	if !ok {
		return nil, fmt.Errorf("unknown IPTV-org source: %s", sourceID)
	}

	if IPTVOrgProviderExists(source.URL) {
		return nil, fmt.Errorf("source already imported: %s", source.Name)
	}

	provider, err := IPTVOrgProvider(sourceID)
	if err != nil {
		return nil, err
	}

	providerID, ok := provider["id.provider"].(string)
	if !ok || providerID == "" {
		return nil, fmt.Errorf("invalid IPTV-org provider ID")
	}

	if Settings.Files.M3U == nil {
		Settings.Files.M3U = make(map[string]interface{})
	}

	Settings.Files.M3U[providerID] = provider

	if err := saveSettings(Settings); err != nil {
		delete(Settings.Files.M3U, providerID)
		return nil, err
	}

	return provider, nil
}

func IPTVOrgRemoveProvider(sourceID string) error {
	source, ok := IPTVOrgGetSource(sourceID)
	if !ok {
		return fmt.Errorf("unknown IPTV-org source: %s", sourceID)
	}

	for id, value := range Settings.Files.M3U {
		provider, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		if provider["file.source"] == source.URL {
			delete(Settings.Files.M3U, id)

			if err := saveSettings(Settings); err != nil {
				return err
			}

			return nil
		}
	}

	return fmt.Errorf("source is not imported: %s", source.Name)
}
