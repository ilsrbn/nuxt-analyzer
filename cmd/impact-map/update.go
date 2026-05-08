package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const updateCacheTTL = 24 * time.Hour

var latestReleaseURL = "https://api.github.com/repos/ilsrbn/nuxt-analyzer/releases/latest"

type latestVersionCache struct {
	CheckedAt time.Time `json:"checked_at"`
	Version   string    `json:"version"`
}

type updateCheckConfig struct {
	CachePath string
	Client    *http.Client
	Stderr    io.Writer
}

type updateCheck struct {
	latest <-chan string
	stderr io.Writer
}

func startBackgroundUpdateCheck(ctx context.Context, cfg updateCheckConfig) *updateCheck {
	stderr := cfg.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	latest := make(chan string, 1)
	check := &updateCheck{latest: latest, stderr: stderr}

	cachePath := cfg.CachePath
	if cachePath == "" {
		cachePath = defaultLatestVersionCachePath()
	}

	if cached, ok, err := readFreshLatestVersionCache(cachePath, updateCacheTTL); err == nil && ok {
		if isNewerVersion(version, cached.Version) {
			latest <- cached.Version
		}
		return check
	}

	client := cfg.Client
	if client == nil {
		client = http.DefaultClient
	}

	go func() {
		defer close(latest)

		checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		tag, err := fetchLatestVersion(checkCtx, client)
		if err != nil || tag == "" {
			return
		}

		_ = writeLatestVersionCache(cachePath, latestVersionCache{
			CheckedAt: time.Now(),
			Version:   tag,
		})

		if isNewerVersion(version, tag) {
			latest <- tag
		}
	}()

	return check
}

func (c *updateCheck) PrintNotice() {
	if c == nil || c.latest == nil {
		return
	}

	select {
	case latest, ok := <-c.latest:
		if ok && latest != "" {
			fmt.Fprintf(c.stderr, "impact-map %s: update available %s. Run `impact-map upgrade`\n", version, latest)
		}
	default:
	}
}

func defaultLatestVersionCachePath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(cacheDir, "impact-map", "latest_version")
}

func readFreshLatestVersionCache(path string, ttl time.Duration) (latestVersionCache, bool, error) {
	if path == "" {
		return latestVersionCache{}, false, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return latestVersionCache{}, false, nil
		}
		return latestVersionCache{}, false, err
	}

	var cached latestVersionCache
	if err := json.Unmarshal(data, &cached); err != nil {
		return latestVersionCache{}, false, err
	}
	if cached.Version == "" || cached.CheckedAt.IsZero() {
		return latestVersionCache{}, false, nil
	}
	if time.Since(cached.CheckedAt) > ttl {
		return latestVersionCache{}, false, nil
	}

	return cached, true, nil
}

func writeLatestVersionCache(path string, cached latestVersionCache) error {
	if path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(cached)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func fetchLatestVersion(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "impact-map/"+version)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("github latest release: %s", resp.Status)
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.TagName == "" {
		return "", fmt.Errorf("github latest release: missing tag_name")
	}

	return payload.TagName, nil
}

func isNewerVersion(current, latest string) bool {
	current = strings.TrimSpace(current)
	latest = strings.TrimSpace(latest)
	if latest == "" || normalizeVersion(current) == normalizeVersion(latest) {
		return false
	}
	if current == "" || current == "dev" {
		return true
	}

	currentParts, currentOK := semanticVersionParts(current)
	latestParts, latestOK := semanticVersionParts(latest)
	if !currentOK || !latestOK {
		return current != latest
	}

	for i := range latestParts {
		if latestParts[i] > currentParts[i] {
			return true
		}
		if latestParts[i] < currentParts[i] {
			return false
		}
	}
	return false
}

func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func semanticVersionParts(v string) ([3]int, bool) {
	var parts [3]int
	v = normalizeVersion(v)
	if before, _, ok := strings.Cut(v, "-"); ok {
		v = before
	}
	fields := strings.Split(v, ".")
	if len(fields) != 3 {
		return parts, false
	}
	for i, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}
