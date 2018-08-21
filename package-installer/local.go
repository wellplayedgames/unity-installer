package packageinstaller

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	shellquote "github.com/kballard/go-shellquote"
	"github.com/wellplayedgames/unity-installer/releases"
)

const (
	// ModulesFile is the file used to store current module state.
	ModulesFile = "modules.json"
)

// PackageInstaller represents an object capable of installing packages.
type PackageInstaller interface {
	io.Closer

	InstallPackage(packagePath string, destination string, options releases.InstallOptions) error
	StoreModules(destination string, modules []releases.ModuleRelease) error
}

type localInstaller struct{}

func NewLocalInstaller() PackageInstaller {
	return &localInstaller{}
}

func (i *localInstaller) Close() error {
	return nil
}

func (i *localInstaller) StoreModules(destination string, modules []releases.ModuleRelease) error {
	path := filepath.Join(destination, ModulesFile)
	b, err := json.MarshalIndent(&modules, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, b, os.ModePerm)
}

// InstallPackage installs a single Unity package.
func (i *localInstaller) InstallPackage(packagePath string, destination string, options releases.InstallOptions) error {
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
