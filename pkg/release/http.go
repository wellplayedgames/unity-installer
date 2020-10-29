package release

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type httpReleases struct {
	Official []EditorRelease `json:"official"`
	Beta     []EditorRelease `json:"beta"`
}

// HTTPReleaseSource can be used to fetch releases from the official Unity
// archives.
type HTTPReleaseSource struct {
	HTTPClient                *http.Client
	PublishedVersionsEndpoint string
	GAArchiveURL              string
	TestingArchiveURL         string
}

func joinSlash(a, b string) string {
	prefixSlash := strings.HasSuffix(a, "/")
	suffixSlash := strings.HasPrefix(b, "/")

	if prefixSlash != suffixSlash {
		return a + b
	}

	if !prefixSlash {
		return fmt.Sprintf("%s/%s", a, b)
	}

	return a + b[1:]
}

func (s *HTTPReleaseSource) fetch(platform string) (*httpReleases, error) {
	url := joinSlash(s.PublishedVersionsEndpoint, fmt.Sprintf("releases-%s.json", platform))
	resp, err := s.HTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() {
		if nextErr := resp.Body.Close(); err == nil {
			err = nextErr
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error fetching releases: %d", resp.StatusCode)
	}

	d := json.NewDecoder(resp.Body)

	releases := &httpReleases{}
	err = d.Decode(releases)
	if err != nil {
		return nil, err
	}

	return releases, nil
}

func (s *HTTPReleaseSource) fetchSpecificRelease(baseURL, platform, version, revision string) (*EditorRelease, error) {
	suffix := platform
	if platform == "win32" {
		suffix = "win"
	}

	url := joinSlash(baseURL, fmt.Sprintf("%s/unity-%s-%s.ini", revision, version, suffix))
	return parseArchive(s.HTTPClient, url, platform)
}

// FetchReleases implements the Source interface.
func (s *HTTPReleaseSource) FetchReleases(platform string, includeBeta bool) (Releases, error) {
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

// FetchRelease implements the Source interface.
func (s *HTTPReleaseSource) FetchRelease(platform, version, revision string) (*EditorRelease, error) {
	isTesting := strings.ContainsAny(version, "ab")

	if revision == "" {
		releases, err := s.FetchReleases(platform, isTesting)
		if err != nil {
			return nil, err
		}

		for _, release := range releases {
			if strings.HasPrefix(release.Version, version) {
				return release, nil
			}
		}

		return nil, fmt.Errorf("no such version: %s %s", version, platform)
	}

	baseUrl := s.GAArchiveURL
	if isTesting {
		baseUrl = s.TestingArchiveURL
	}

	return s.fetchSpecificRelease(baseUrl, platform, version, revision)
}
