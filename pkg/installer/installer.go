package installer

import (
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"

	packageinstaller "github.com/wellplayedgames/unity-installer/pkg/package-installer"
	"github.com/wellplayedgames/unity-installer/pkg/release"
)

// UnityInstaller represents a Unity Installer which can install editors and editor modules.
type UnityInstaller interface {
	io.Closer

	InstallEditor(platform string, installer packageinstaller.PackageInstaller, spec *release.EditorRelease) error
	InstallModule(installer packageinstaller.PackageInstaller, editorVersion string, spec *release.ModuleRelease) error

	CheckEditorVersion(editorVersion string) (bool, []release.ModuleRelease, error)
}

type simpleInstaller struct {
	logger     logr.Logger
	httpClient *http.Client
	editorDir  string
	tempDir    string
}

// NewSimpleInstaller creates a Unity Installer which downloads packages to a
// temporary directory every install.
func NewSimpleInstaller(logger logr.Logger, editorDir, tempDir string, client *http.Client) (UnityInstaller, error) {
	i := &simpleInstaller{logger, client, editorDir, tempDir}
	return i, nil
}

func (i *simpleInstaller) Close() error {
	return nil
}

func (i *simpleInstaller) downloadPackage(pkg *release.Package) (string, error) {
	i.logger.Info("downloading package", "package", pkg.DownloadURL)

	_, fileName := path.Split(pkg.DownloadURL)
	targetPath := filepath.Join(i.tempDir, fileName)

	resp, err := i.httpClient.Get(pkg.DownloadURL)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			i.logger.Error(err, "failed to close download body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error fetching package: %d", resp.StatusCode)
	}

	target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := target.Close(); err != nil {
			i.logger.Error(err, "failed to close target file")
		}
	}()

	_, err = io.Copy(target, resp.Body)
	return targetPath, err
}

func (i *simpleInstaller) InstallEditor(platform string, packageInstaller packageinstaller.PackageInstaller, spec *release.EditorRelease) error {
	targetPath := filepath.Join(i.editorDir, spec.Version)

	packagePath, err := i.downloadPackage(&spec.Package)
	if err == nil {
		installOptions := release.InstallOptions{
			Destination: &targetPath,
		}

		if platform == "darwin" {
			renameFrom := "{UNITY_PATH}/Unity"
			renameTo := "{UNITY_PATH}"
			installOptions.RenameFrom = &renameFrom
			installOptions.RenameTo = &renameTo
		}

		err = packageInstaller.InstallPackage(packagePath, targetPath, installOptions)
	}

	if err == nil {
		mods := make([]release.ModuleRelease, len(spec.Modules))

		for idx, mod := range spec.Modules {
			mod.Selected = false
			mods[idx] = mod
		}

		err = packageInstaller.StoreModules(targetPath, mods)
	}

	return err
}

func (i *simpleInstaller) InstallModule(packageInstaller packageinstaller.PackageInstaller, editorVersion string, spec *release.ModuleRelease) error {
	_, existingModules, err := i.CheckEditorVersion(editorVersion)
	if err != nil {
		return err
	}

	targetPath := filepath.Join(i.editorDir, editorVersion)

	packagePath, err := i.downloadPackage(&spec.Package)
	if err == nil {
		err = packageInstaller.InstallPackage(packagePath, targetPath, spec.InstallOptions)
	}

	// Update modules
	if err == nil {
		modMap := map[string]release.ModuleRelease{}

		for _, mod := range existingModules {
			modMap[mod.ID] = mod
		}

		thisMod := *spec
		thisMod.Selected = true
		modMap[spec.ID] = thisMod

		mods := make([]release.ModuleRelease, 0, len(modMap))

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

	macPath := filepath.Join(editorDir, "Unity.app")
	if checkFileExists(macPath) {
		return true
	}

	return false
}

func (i *simpleInstaller) CheckEditorVersion(editorVersion string) (bool, []release.ModuleRelease, error) {
	editorDir := filepath.Join(i.editorDir, editorVersion)
	if !checkEditorDirectory(editorDir) {
		return false, nil, nil
	}

	modulesPath := filepath.Join(editorDir, packageinstaller.ModulesFile)
	file, err := os.Open(modulesPath)
	if err != nil {
		return true, nil, nil
	}

	defer func() {
		if err := file.Close(); err != nil {
			i.logger.Error(err, "failed to close editor metadata")
		}
	}()
	d := json.NewDecoder(file)

	var modules []release.ModuleRelease
	err = d.Decode(&modules)
	if err != nil {
		return true, nil, err
	}

	return true, modules, nil
}
