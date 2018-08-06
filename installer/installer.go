package installer

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	shellquote "github.com/kballard/go-shellquote"
	"github.com/wellplayedgames/unity-hub/installer/releases"
)

// UnityInstaller represents a Unity Installer which can install editors and editor modules.
type UnityInstaller interface {
	io.Closer

	InstallEditor(editorVersion string) error
	InstallModule(editorVersion string, moduleID string) error

	CheckEditorVersion(editorVersion string) (bool, []string, error)
}

type simpleInstaller struct {
	httpClient *http.Client
	editorDir  string
	tempDir    string
	releases   *releases.Releases
}

// NewSimpleInstaller creates a Unity Installer which downloads packages to a
// temporary directory every install.
func NewSimpleInstaller(editorDir string, releases *releases.Releases, client *http.Client) (UnityInstaller, error) {
	tempDir, err := ioutil.TempDir("", "UnityInstaller")
	if err != nil {
		return nil, err
	}

	i := &simpleInstaller{client, editorDir, tempDir, releases}
	return i, nil
}

// NewInstaller creates a UnityInstaller with default configuration.
func NewInstaller(editorDir string) (UnityInstaller, error) {
	releaseManifest, err := releases.DefaultReleaseSource.FetchReleases("win32")
	if err != nil {
		return nil, err
	}

	unityInstaller, err := NewSimpleInstaller(editorDir, releaseManifest, http.DefaultClient)
	if err != nil {
		return nil, err
	}

	return unityInstaller, nil
}

func (i *simpleInstaller) Close() error {
	return os.RemoveAll(i.tempDir)
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

func (i *simpleInstaller) findEditorRelease(editorVersion string) (*releases.EditorRelease, error) {
	for idx := range i.releases.Official {
		editor := &i.releases.Official[idx]
		if editor.Version == editorVersion {
			return editor, nil
		}
	}

	return nil, fmt.Errorf("no such editor version %s", editorVersion)
}

func (i *simpleInstaller) findModuleRelease(editorVersion string, moduleID string) (*releases.EditorRelease, *releases.ModuleRelease, error) {
	editorRelease, err := i.findEditorRelease(editorVersion)
	if err != nil {
		return nil, nil, err
	}

	for idx := range editorRelease.Modules {
		moduleRelease := &editorRelease.Modules[idx]
		if moduleRelease.ID == moduleID {
			return editorRelease, moduleRelease, nil
		}
	}

	return nil, nil, fmt.Errorf("no such module %s", moduleID)
}

func (i *simpleInstaller) InstallEditor(editorVersion string) error {
	targetPath := filepath.Join(i.editorDir, editorVersion)

	editorRelease, err := i.findEditorRelease(editorVersion)
	if err != nil {
		return err
	}

	packagePath, err := i.downloadPackage(&editorRelease.Package)
	if err == nil {
		err = InstallPackage(packagePath, targetPath, releases.InstallOptions{
			Destination: &targetPath,
		})
	}

	return err
}

func (i *simpleInstaller) InstallModule(editorVersion string, moduleID string) error {
	targetPath := filepath.Join(i.editorDir, editorVersion)

	_, moduleRelease, err := i.findModuleRelease(editorVersion, moduleID)
	if err != nil {
		return err
	}

	packagePath, err := i.downloadPackage(&moduleRelease.Package)
	if err == nil {
		err = InstallPackage(packagePath, targetPath, moduleRelease.InstallOptions)
	}

	return err
}

func (i *simpleInstaller) CheckEditorVersion(editorVersion string) (bool, []string, error) {
	editorDir := filepath.Join(i.editorDir, editorVersion)
	exePath := filepath.Join(editorDir, "Editor", "Unity.exe")

	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		return false, nil, nil
	}

	modulesPath := filepath.Join(editorDir, "modules.json")
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

	installedModules := []string{}

	for _, module := range modules {
		if module.Selected {
			installedModules = append(installedModules, module.ID)
		}
	}

	return true, installedModules, nil
}

// InstallPackage installs a single Unity package.
func InstallPackage(packagePath string, destination string, options releases.InstallOptions) error {
	fmt.Printf("installing %s...\n", packagePath)

	if strings.HasSuffix(packagePath, ".zip") {
		return installZip(packagePath, destination)
	}

	return installExe(packagePath, destination, options)
}

func installZip(packagePath string, destination string) error {
	r, err := zip.OpenReader(packagePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fr, err := f.Open()
		if err != nil {
			return err
		}

		targetPath := filepath.Join(destination, f.Name)

		if f.FileInfo().IsDir() {
			err = os.MkdirAll(targetPath, os.ModePerm)
			if err != nil {
				return err
			}
			continue
		}

		parentDir := filepath.Dir(targetPath)
		err = os.MkdirAll(parentDir, os.ModePerm)
		if err != nil {
			return err
		}

		fw, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		err = func() error {
			defer fw.Close()
			_, err := io.Copy(fw, fr)
			return err
		}()
		if err != nil {
			return err
		}
	}

	return nil
}

func installExe(packagePath string, destination string, options releases.InstallOptions) error {
	var args []string
	var err error

	if options.Command != nil {
		cmdString := strings.Replace(*options.Command, "{INSTDIR}", destination, -1)
		args, err = shellquote.Split(cmdString)
		if err != nil {
			return err
		}
	} else {
		args = append(args, "/S")

		if options.Destination != nil {
			destPath := strings.Replace(*options.Destination, "{UNITY_PATH}", destination, -1)
			args = append(args, fmt.Sprintf("/D=%s", destPath))
		}
	}

	fmt.Printf("running %s %s\n", packagePath, args)

	cmd := exec.Command(packagePath, args...)
	err = cmd.Run()
	if err != nil {
		out, _ := cmd.CombinedOutput()
		errText := ""

		if out != nil {
			errText = string(out)
		}

		fmt.Fprint(os.Stderr, errText)
		return err
	}

	return nil
}
