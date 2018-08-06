package releases

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// ReleaseSource provides a means of listing released Unity versions with
// metadata.
type ReleaseSource interface {
	FetchReleases(platform string) (*Releases, error)
}

// InstallOptions provides the options to configure a package to install.
type InstallOptions struct {
	Command     *string `json:"cmd,omitempty"`
	Destination *string `json:"destination,omitempty"`
}

// Package represents a single package which will be installed as part of a
// Unity install.
type Package struct {
	InstallOptions `json:",inline"`
	DownloadURL    string `json:"downloadUrl"`
	DownloadSize   int64  `json:"downloadSize"`
	InstalledSize  int64  `json:"installedSize"`
	Checksum       string `json:"checksum,omitempty"`
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

// Releases lists all available releases at a point in time.
type Releases struct {
	Official []EditorRelease `json:"official"`
}

type httpReleaseSource struct {
	client  *http.Client
	baseURL string

	releases map[string]*Releases
	lock     sync.Mutex
}

// NewHTTPReleaseSource returns a new ReleaseSource which uses the Unity Hub API.
func NewHTTPReleaseSource(client *http.Client, baseURL string) ReleaseSource {
	return &httpReleaseSource{
		client:   client,
		baseURL:  baseURL,
		releases: map[string]*Releases{},
	}
}

func (s *httpReleaseSource) FetchReleases(platform string) (*Releases, error) {
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

	releases := &Releases{}
	err = d.Decode(releases)
	if err != nil {
		return nil, err
	}

	s.releases[platform] = releases
	return releases, nil
}
