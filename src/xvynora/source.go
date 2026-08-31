package xvynora

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	IPTVOrgChannels = "https://iptv-org.github.io/api/channels.json"
	IPTVOrgStreams  = "https://iptv-org.github.io/api/streams.json"
)

type Channel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Country  string `json:"country"`
	Categories []string `json:"categories"`
	Logo     string `json:"logo"`
}

type Stream struct {
	ID      string `json:"channel"`
	URL     string `json:"url"`
	HTTPRef string `json:"http_referrer"`
	UserAgent string `json:"user_agent"`
}

var client = &http.Client{
	Timeout: 30 * time.Second,
}

func fetchJSON(url string, target any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "XVynora-IPTV/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func LoadChannels() ([]Channel, error) {
	var channels []Channel

	if err := fetchJSON(IPTVOrgChannels, &channels); err != nil {
		return nil, err
	}

	return channels, nil
}

func LoadStreams() ([]Stream, error) {
	var streams []Stream

	if err := fetchJSON(IPTVOrgStreams, &streams); err != nil {
		return nil, err
	}

	// Only retain usable HTTP(S) streams.
	result := make([]Stream, 0, len(streams))

	for _, stream := range streams {
		if strings.HasPrefix(stream.URL, "http://") ||
			strings.HasPrefix(stream.URL, "https://") {
			result = append(result, stream)
		}
	}

	return result, nil
}
