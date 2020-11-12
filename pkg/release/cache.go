package release

import (
	"fmt"
	"sync"
)

// Cache caches release fetches to save time.
type Cache struct {
	inner     Source
	releases  map[string]Releases
	revisions map[string]*EditorRelease
	lock      sync.Mutex
}
var _ Source = (*Cache)(nil)

// NewCache creates a release source which caches another release source.
func NewCache(inner Source) *Cache {
	return &Cache{
		inner:    inner,
		releases: map[string]Releases{},
		revisions: map[string]*EditorRelease{},
	}
}

// FetchReleases implements the Source interface.
func (c *Cache) FetchReleases(platform string, includeBeta bool) (Releases, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if existing, ok := c.releases[platform]; ok {
		return existing, nil
	}

	releases, err := c.inner.FetchReleases(platform, includeBeta)
	if err != nil {
		return nil, err
	}

	c.releases[platform] = releases
	return releases, nil
}

// FetchRelease implements the Source interface.
func (c *Cache) FetchRelease(platform, version, revision string) (*EditorRelease, error) {
	key := fmt.Sprintf("%s@%s", version, revision)

	c.lock.Lock()
	defer c.lock.Unlock()

	if existing, ok := c.revisions[key]; ok {
		return existing, nil
	}

	release, err := c.inner.FetchRelease(platform, version, revision)
	if err != nil {
		return nil, err
	}

	c.revisions[key] = release
	return release, nil
}
