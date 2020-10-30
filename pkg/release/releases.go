package release

import (
	"net/http"
)

var (
	defaultPublishedReleasesEndpoint = "https://public-cdn.cloud.unity3d.com/hub/prod/"
	defaultGAArchiveEndpoint         = "https://download.unity3d.com/download_unity/"
	defaultTestingArchiveEndpoint    = "http://beta.unity3d.com/download/"

	// DefaultReleaseSource fetches releases from the Unity Hub endpoints.
	DefaultReleaseSource = HTTPReleaseSource{
		HTTPClient:                http.DefaultClient,
		PublishedVersionsEndpoint: defaultPublishedReleasesEndpoint,
		GAArchiveURL:              defaultGAArchiveEndpoint,
		TestingArchiveURL:         defaultTestingArchiveEndpoint,
	}
)

// Source provides a means of listing released Unity versions with
// metadata.
type Source interface {
	FetchReleases(platform string, includeBeta bool) (Releases, error)
	FetchRelease(platform, version, revision string) (*EditorRelease, error)
}

// InstallOptions provides the options to configure a package to install.
type InstallOptions struct {
	Command     *string `json:"cmd,omitempty"`
	Destination *string `json:"destination,omitempty"`

	// Advanced install options
	RenameFrom *string `json:"renameFrom"`
	RenameTo   *string `json:"renameTo"`
	Checksum   string  `json:"checksum,omitempty"`
}

// Package represents a single package which will be installed as part of a
// Unity install.
type Package struct {
	InstallOptions `json:",inline"`
	Version        string `json:"version"`
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
