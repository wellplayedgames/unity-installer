package packageinstaller

import (
	"archive/zip"
	"encoding/json"
	"errors"
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
	unityPath := destination
	fmt.Printf("installing %s...\n", packagePath)

	if options.Destination != nil {
		destination = filepath.Clean(strings.ReplaceAll(*options.Destination, "{UNITY_PATH}", unityPath))
	}

	os.MkdirAll(destination, os.ModePerm)

	err := func() error {
		if strings.HasSuffix(packagePath, ".zip") {
			return installZip(packagePath, destination)
		}

		if strings.HasSuffix(packagePath, ".pkg") {
			return installPkg(packagePath, destination)
		}

		return installExe(packagePath, destination, options)
	}()

	if err == nil && options.RenameFrom != nil && options.RenameTo != nil {
		renameFrom := filepath.Clean(strings.ReplaceAll(*options.RenameFrom, "{UNITY_PATH}", unityPath))
		renameTo := filepath.Clean(strings.ReplaceAll(*options.RenameTo, "{UNITY_PATH}", unityPath))

		fmt.Printf("moving %s to %s...\n", renameFrom, renameTo)

		renameToDir := filepath.Dir(renameTo)
		os.MkdirAll(renameToDir, os.ModePerm)

		rel := ""
		rel, err = filepath.Rel(renameTo, renameFrom)
		rejoinName := filepath.Join(renameTo, rel)
		fmt.Printf("REJOIN %s %s...\n", rel, rejoinName)

		// If renameTo is a parent of renameFrom, we need some special work.
		if err == nil && rejoinName == renameFrom {
			tmpPath := filepath.Join(renameToDir, "_tmp")

			fmt.Printf("using tmp rename %s...\n", tmpPath)
			err = os.Rename(renameFrom, tmpPath)
			if err != nil {
				fmt.Printf("Failed to rename %s -> %s\n", renameFrom, tmpPath)
				return err
			}

			renameFrom = tmpPath
		}

		err = os.Remove(renameTo)
		if os.IsNotExist(err) {
			err = nil
		} else if err != nil {
			fmt.Printf("Failed to remove %s with error %s\n", renameTo, err)
			return err
		}

		err = os.Rename(renameFrom, renameTo)
	}

	return err
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

		mode := f.Mode() | 0666
		fw, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
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

func findPackage(dir string) (string, error) {
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".pkg.tmp") {
			return name, nil
		}
	}

	return "", errors.New("could not find Payload")
}

func installPkg(packagePath, destination string) error {
	tmpPath, err := ioutil.TempDir("", "unity-installer")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpPath)

	// First, extract the package file.
	cmd := exec.Command("/usr/bin/xar", "-xf", packagePath, "-C", tmpPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()

	// Then extract package Payload
	tmpPkgPath := ""
	if err == nil {
		tmpPkgPath, err = findPackage(tmpPath)
	}

	if err == nil {
		payloadPath := filepath.Join(tmpPath, tmpPkgPath, "Payload")
		cmd := exec.Command("/usr/bin/tar", "-C", destination, "-zmxf", payloadPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
	}

	return err
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
