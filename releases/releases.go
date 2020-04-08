package releases

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

var (
	defaultReleasesEndpoint = "https://public-cdn.cloud.unity3d.com/hub/prod/"

	// DefaultReleaseSource fetches releases from the Unity Hub endpoints.
	DefaultReleaseSource = NewHTTPReleaseSource(http.DefaultClient, defaultReleasesEndpoint)
)

// ReleaseSource provides a means of listing released Unity versions with
// metadata.
type ReleaseSource interface {
	FetchReleases(platform string, includeBeta bool) (Releases, error)
}

// InstallOptions provides the options to configure a package to install.
type InstallOptions struct {
	Command     *string `json:"cmd,omitempty"`
	Destination *string `json:"destination,omitempty"`

	// Advanced install options
	RenameFrom *string `json:"renameFrom"`
	RenameTo *string `json:"renameTo"`
	Checksum       string `json:"checksum,omitempty"`
}

// Package represents a single package which will be installed as part of a
// Unity install.
type Package struct {
	InstallOptions `json:",inline"`
	DownloadURL    string `json:"downloadUrl"`
	DownloadSize   int64  `json:"downloadSize"`
	InstalledSize  int64  `json:"installedSize"`
}

// ModuleRelease represents an optional Unity module tied to a specific editor
// version.
type ModuleRelease struct {
	Package     `json:",inline"`
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	Visible     bool   `json:"visible"`
	Selected    bool   `json:"selected"`
}

// EditorRelease represents a single release of a Unity version.
type EditorRelease struct {
	Package `json:",inline"`

	Version string `json:"version"`
	LTS     bool   `json:"lts"`

	Modules []ModuleRelease `json:"modules"`
}

// FindModule returns the first module with a given ID or nil.
func (r *EditorRelease) FindModule(id string) *ModuleRelease {
	for idx := range r.Modules {
		m := &r.Modules[idx]
		if m.ID == id {
			return m
		}
	}

	return nil
}

// Releases lists all available releases at a point in time.
type Releases map[string]*EditorRelease

type httpReleases struct {
	Official []EditorRelease `json:"official"`
	Beta     []EditorRelease `json:"beta"`
}

type httpReleaseSource struct {
	client  *http.Client
	baseURL string

	releases map[string]*httpReleases
	lock     sync.Mutex
}

// NewHTTPReleaseSource returns a new ReleaseSource which uses the Unity Hub API.
func NewHTTPReleaseSource(client *http.Client, baseURL string) ReleaseSource {
	return &httpReleaseSource{
		client:   client,
		baseURL:  baseURL,
		releases: map[string]*httpReleases{},
	}
}

func (s *httpReleaseSource) fetch(platform string) (*httpReleases, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if existing, ok := s.releases[platform]; ok {
		return existing, nil
	}

	url := fmt.Sprintf("%sreleases-%s.json", s.baseURL, platform)
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error fetching releases: %d", resp.StatusCode)
	}

	d := json.NewDecoder(resp.Body)

	releases := &httpReleases{}
	err = d.Decode(releases)
	if err != nil {
		return nil, err
	}

	s.releases[platform] = releases
	return releases, nil
}

func (s *httpReleaseSource) FetchReleases(platform string, includeBeta bool) (Releases, error) {
	releases, err := s.fetch(platform)
	if err != nil {
		return nil, err
	}

	ret := Releases{}

	for idx := range releases.Official {
		v := &releases.Official[idx]
		ret[v.Version] = v
	}

	if includeBeta {
		for idx := range releases.Beta {
			v := &releases.Beta[idx]
			ret[v.Version] = v
		}
	}

	return ret, nil
}
