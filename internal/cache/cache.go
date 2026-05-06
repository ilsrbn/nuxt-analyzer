package cache

// Cache is the minimal cache contract used by the analyzer.
// It is intentionally small so the stub can be replaced later with a
// filesystem, Redis, or content-addressed implementation without touching
// callers.
type Cache interface {
	Get(key string) ([]byte, bool)
	Set(key string, data []byte) error
}

// NoopCache is a stub implementation that always misses and never persists.
// It keeps the package safe to wire in before a real cache strategy exists.
type NoopCache struct{}

func (NoopCache) Get(_ string) ([]byte, bool) { return nil, false }

func (NoopCache) Set(_ string, _ []byte) error { return nil }

// CacheKey combines the base and head refs into a deterministic stub key.
// The simple separator is enough for now; future implementations can switch to
// a richer strategy if ref formats or cache scope need to change.
func CacheKey(base, head string) string {
	return base + ":" + head
}
