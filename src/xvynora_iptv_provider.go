package src

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func xvynoraIPTVProviderPath(id string) string {
	return filepath.Join(System.Folder.Data, "xvynora-"+id+".m3u")
}

func xvynoraIPTVImport(id string) error {
	source, ok := xvynoraIPTVGetSource(id)
	if !ok {
		return fmt.Errorf("unknown IPTV-org source: %s", id)
	}

	data, err := xvynoraIPTVDownload(source.URL)
	if err != nil {
		return err
	}

	if len(data) == 0 || !strings.Contains(string(data), "#EXTM3U") {
		return fmt.Errorf("invalid M3U returned by IPTV-org")
	}

	path := xvynoraIPTVProviderPath(id)

	if err := os.MkdirAll(System.Folder.Data, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	return nil
}

func xvynoraIPTVRemove(id string) error {
	source, ok := xvynoraIPTVGetSource(id)
	if !ok {
		return fmt.Errorf("unknown IPTV-org source: %s", id)
	}

	_ = source

	path := xvynoraIPTVProviderPath(id)

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func xvynoraIPTVUpdate(id string) error {
	return xvynoraIPTVImport(id)
}

func xvynoraIPTVProviderStatus(id string) map[string]interface{} {
	source, ok := xvynoraIPTVGetSource(id)

	result := map[string]interface{}{
		"id":       id,
		"imported": false,
	}

	if !ok {
		result["error"] = "unknown source"
		return result
	}

	result["name"] = source.Name
	result["url"] = source.URL

	info, err := os.Stat(xvynoraIPTVProviderPath(id))
	if err == nil {
		result["imported"] = true
		result["updated"] = info.ModTime().Format(time.RFC3339)
		result["size"] = info.Size()
	}

	return result
}
