package release

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-ini/ini"
)

const (
	editorModuleName = "Unity"
)

type archiveModule struct {
	Title         string  `ini:"title"`
	Description   string  `ini:"description"`
	URL           string  `ini:"url"`
	MD5           string  `ini:"md5"`
	InstalledSize int64   `ini:"installedsize"`
	DownloadSize  int64   `ini:"size"`
	Command       *string `ini:"cmd"`
}

func fetchIni(c *http.Client, url string) (*ini.File, error) {
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}

	defer func() {
		if cerr := resp.Body.Close(); err == nil {
			err = cerr
		}
	}()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad status %d fetching %s: %s", resp.StatusCode, url, string(bodyBytes))
	}

	file, err := ini.Load(resp.Body)
	return file, err
}

func parseArchive(c *http.Client, archiveURL, platform string) (*EditorRelease, error) {
	meta, err := fetchIni(c, archiveURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download archive metadata: %w", err)
	}

	baseURL, _ := path.Split(archiveURL)
	modules := map[string]*archiveModule{}
	for _, section := range meta.Sections() {
		var module archiveModule
		if err := section.MapTo(&module); err != nil {
			return nil, fmt.Errorf("failed to parse module %s: %w", section.Name(), err)
		}
		modules[section.Name()] = &module
	}

	editorModule := modules[editorModuleName]
	if editorModule == nil {
		return nil, fmt.Errorf("missing Unity section in archive")
	}

	delete(modules, editorModuleName)

	var release EditorRelease
	hydratePackage(baseURL, &release.Package, editorModule)

	for moduleName, src := range modules {
		var dest ModuleRelease
		hydrateArchiveModule(platform, baseURL, &dest, moduleName, src)
		release.Modules = append(release.Modules, dest)
	}

	generateAndroidModules(&release, platform)
	return &release, nil
}

func hydratePackage(baseURL string, dest *Package, src *archiveModule) {
	url := src.URL
	if !strings.Contains(url, "://") {
		url = joinSlash(baseURL, url)
	}

	dest.Command = src.Command
	dest.Checksum = src.MD5
	dest.DownloadURL = url
}

func hydrateArchiveModule(platform, baseURL string, dest *ModuleRelease, moduleName string, src *archiveModule) {
	hydratePackage(baseURL, &dest.Package, src)

	lowerName := strings.ToLower(moduleName)
	ext := filepath.Ext(src.URL)
	dest.ID = lowerName
	dest.Name = src.Title
	dest.Description = src.Description
	dest.Destination = moduleDestination(platform, lowerName, ext)
}
