package installer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"

	packageinstaller "github.com/wellplayedgames/unity-installer/pkg/package-installer"
	"github.com/wellplayedgames/unity-installer/pkg/releases"
)

// UnityInstaller represents a Unity Installer which can install editors and editor modules.
type UnityInstaller interface {
	io.Closer

	InstallEditor(installer packageinstaller.PackageInstaller, spec *releases.EditorRelease) error
	InstallModule(installer packageinstaller.PackageInstaller, editorVersion string, spec *releases.ModuleRelease) error

	CheckEditorVersion(editorVersion string) (bool, []releases.ModuleRelease, error)
}

type simpleInstaller struct {
	httpClient *http.Client
	editorDir  string
	tempDir    string
}

// NewSimpleInstaller creates a Unity Installer which downloads packages to a
// temporary directory every install.
func NewSimpleInstaller(editorDir, tempDir string, client *http.Client) (UnityInstaller, error) {
	i := &simpleInstaller{client, editorDir, tempDir}
	return i, nil
}

func (i *simpleInstaller) Close() error {
	return nil
}

func (i *simpleInstaller) downloadPackage(pkg *releases.Package) (string, error) {
	fmt.Printf("downloading %s...\n", pkg.DownloadURL)

	_, fname := path.Split(pkg.DownloadURL)
	targetPath := filepath.Join(i.tempDir, fname)

	resp, err := i.httpClient.Get(pkg.DownloadURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error fetching package: %d", resp.StatusCode)
	}

	target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return "", err
	}
	defer target.Close()

	_, err = io.Copy(target, resp.Body)
	return targetPath, err
}

func (i *simpleInstaller) InstallEditor(packageInstaller packageinstaller.PackageInstaller, spec *releases.EditorRelease) error {
	targetPath := filepath.Join(i.editorDir, spec.Version)

	packagePath, err := i.downloadPackage(&spec.Package)
	if err == nil {
		err = packageInstaller.InstallPackage(packagePath, targetPath, releases.InstallOptions{
			Destination: &targetPath,
		})
	}

	if err == nil {
		mods := make([]releases.ModuleRelease, len(spec.Modules))

		for idx, mod := range spec.Modules {
			mod.Selected = false
			mods[idx] = mod
		}

		err = packageInstaller.StoreModules(targetPath, mods)
	}

	return err
}

func (i *simpleInstaller) InstallModule(packageInstaller packageinstaller.PackageInstaller, editorVersion string, spec *releases.ModuleRelease) error {
	hasEditor, existingModules, err := i.CheckEditorVersion(editorVersion)
	if err != nil {
		return err
	}

	if !hasEditor {
		return errors.New("editor not installed")
	}

	targetPath := filepath.Join(i.editorDir, editorVersion)

	packagePath, err := i.downloadPackage(&spec.Package)
	if err == nil {
		err = packageInstaller.InstallPackage(packagePath, targetPath, spec.InstallOptions)
	}

	// Update modules
	if err == nil {
		modMap := map[string]releases.ModuleRelease{}

		for _, mod := range existingModules {
			modMap[mod.ID] = mod
		}

		thisMod := *spec
		thisMod.Selected = true
		modMap[spec.ID] = thisMod

		mods := make([]releases.ModuleRelease, 0, len(modMap))

		for _, v := range modMap {
			mods = append(mods, v)
		}

		err = packageInstaller.StoreModules(targetPath, mods)
	}

	return err
}

func checkFileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}

	return false
}

func checkEditorDirectory(editorDir string) bool {
	winPath := filepath.Join(editorDir, "Editor", "Unity.exe")
	if checkFileExists(winPath) {
		return true
	}

	macPath := filepath.Join(editorDir, "Unity", "Unity.app")
	if checkFileExists(macPath) {
		return true
	}

	return false
}

func (i *simpleInstaller) CheckEditorVersion(editorVersion string) (bool, []releases.ModuleRelease, error) {
	editorDir := filepath.Join(i.editorDir, editorVersion)
	if !checkEditorDirectory(editorDir) {
		return false, nil, nil
	}

	modulesPath := filepath.Join(editorDir, packageinstaller.ModulesFile)
	file, err := os.Open(modulesPath)
	if err != nil {
		return true, nil, nil
	}

	defer file.Close()

	d := json.NewDecoder(file)

	modules := []releases.ModuleRelease{}
	err = d.Decode(&modules)
	if err != nil {
		return true, nil, err
	}

	return true, modules, nil
}
