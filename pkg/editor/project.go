package editor

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/ghodss/yaml"
)

var (
	projectVersionPath   = filepath.Join("ProjectSettings", "ProjectVersion.txt")
	editorRevisionRegexp = regexp.MustCompile(`^([0-9a-z.]+)\s*\(([A-Fa-f0-9]+)\)$`)
)

// ProjectVersion contains information about the supported editor version for
// the project.
type ProjectVersion struct {
	EditorVersion             string `json:"m_EditorVersion"`
	EditorVersionWithRevision string `json:"m_EditorVersionWithRevision"`
}

// VersionAndRevision parses the editor version number and revision from the
// project version file.
func (p *ProjectVersion) VersionAndRevision() (string, string) {
	match := editorRevisionRegexp.FindStringSubmatch(p.EditorVersionWithRevision)
	if match == nil {
		return p.EditorVersion, ""
	}

	return match[1], match[2]
}

// ProjectVersionFromProject attempts to load the editor version information
// given the path to a Unity project.
func ProjectVersionFromProject(projectPath string) (*ProjectVersion, error) {
	pvPath := filepath.Join(projectPath, projectVersionPath)
	f, err := os.Open(pvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open project version: %w", err)
	}
	defer f.Close()

	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", projectVersionPath, err)
	}

	var pv ProjectVersion
	if err := yaml.Unmarshal(bytes, &pv); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", projectVersionPath, err)
	}

	return &pv, nil
}
