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
	"time"

	"github.com/go-logr/logr"
	shellquote "github.com/kballard/go-shellquote"
	"github.com/wellplayedgames/unity-installer/pkg/release"
)

const (
	// ModulesFile is the file used to store current module state.
	ModulesFile = "modules.json"
)

// PackageInstaller represents an object capable of installing packages.
type PackageInstaller interface {
	io.Closer

	InstallPackage(packagePath string, destination string, options release.InstallOptions) error
	StoreModules(destination string, modules []release.ModuleRelease) error
}

func mergeDirectory(src, dest string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		entrySrc := filepath.Join(src, entry.Name())
		entryDest := filepath.Join(dest, entry.Name())

		destInfo, err := os.Stat(entryDest)
		if err == nil && destInfo.IsDir() && srcInfo.IsDir() {
			// Already an existing directory, merge.
			if err := mergeDirectory(entrySrc, entryDest); err != nil {
				return err
			}
		} else if err != nil && !os.IsNotExist(err) {
			// An error other than 'not exists' occurred.
			return err
		} else {
			// Either non-existent or not a directory, just rename
			if err := os.Rename(entrySrc, entryDest); err != nil {
				return err
			}
		}
	}

	err = os.Remove(src)
	if err == nil || os.IsExist(err) {
		return nil
	} else if os.IsExist(err) {
		return nil
	}

	return fmt.Errorf("failed to remove %s: %w", src, err)
}

type localInstaller struct{
	logger logr.Logger
}

func NewLocalInstaller(logger logr.Logger) PackageInstaller {
	return &localInstaller{logger}
}

func (i *localInstaller) Close() error {
	return nil
}

func (i *localInstaller) StoreModules(destination string, modules []release.ModuleRelease) error {
	path := filepath.Join(destination, ModulesFile)
	b, err := json.MarshalIndent(&modules, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, b, os.ModePerm)
}

// InstallPackage installs a single Unity package.
func (i *localInstaller) InstallPackage(packagePath string, destination string, options release.InstallOptions) error {
	unityPath := destination
	startTime := time.Now()
	i.logger.Info("Installing package", "packagePath", packagePath)

	if options.Destination != nil {
		destination = filepath.Clean(strings.ReplaceAll(*options.Destination, "{UNITY_PATH}", unityPath))
	}

	if err := os.MkdirAll(destination, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}

	err := func() error {
		if strings.HasSuffix(packagePath, ".zip") {
			return i.installZip(packagePath, destination)
		}

		if strings.HasSuffix(packagePath, ".pkg") {
			return i.installPkg(packagePath, destination)
		}

		return i.installExe(packagePath, destination, options)
	}()

	if err != nil {
		return err
	}

	if options.RenameFrom != nil && options.RenameTo != nil {
		renameFrom := filepath.Clean(strings.ReplaceAll(*options.RenameFrom, "{UNITY_PATH}", unityPath))
		renameTo := filepath.Clean(strings.ReplaceAll(*options.RenameTo, "{UNITY_PATH}", unityPath))

		statFrom, err := os.Stat(renameFrom)
		if err != nil {
			return fmt.Errorf("cannot access renameFrom: %w", err)
		}

		_, err = os.Stat(renameTo)
		destNotExist := os.IsNotExist(err)
		if _, err := os.Stat(renameTo); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("cannot access renameTo: %w", err)
		}

		renameToDir := filepath.Dir(renameTo)
		if err := os.MkdirAll(renameToDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to make target directory: %w", err)
		}

		if !statFrom.IsDir() || destNotExist {
			i.logger.Info("renaming", "from", renameFrom, "to", renameTo)
			err = os.Rename(renameFrom, renameTo)
		} else {
			i.logger.Info("merging directories", "from", renameFrom, "to", renameTo)
			err = mergeDirectory(renameFrom, renameTo)
		}

		if err != nil {
			return err
		}
	}

	endTime := time.Now()
	delta := endTime.Sub(startTime).String()
	i.logger.Info("installed package", "packagePath", packagePath, "duration", delta)
	return nil
}

func (i *localInstaller) installZip(packagePath string, destination string) error {
	r, err := zip.OpenReader(packagePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			i.logger.Error(err, "failed to close archive")
		}
	}()

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
			defer func() {
				if err := fw.Close(); err != nil {
					i.logger.Error(err, "failed to close archive")
				}
			}()
			_, err := io.Copy(fw, fr)
			return err
		}()
		if err != nil {
			return err
		}
	}

	return nil
}

func (i *localInstaller) findPackage(dir string) (string, error) {
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

func (i *localInstaller) installPkg(packagePath, destination string) error {
	tmpPath, err := ioutil.TempDir("", "unity-installer")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(tmpPath); err != nil {
			i.logger.Error(err, "failed to delete temporary directory")
		}
	}()

	// First, extract the package file.
	cmd := exec.Command("/usr/bin/xar", "-xf", packagePath, "-C", tmpPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()

	// Then extract package Payload
	tmpPkgPath := ""
	if err == nil {
		tmpPkgPath, err = i.findPackage(tmpPath)
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

func (i *localInstaller) installExe(packagePath string, destination string, options release.InstallOptions) error {
	var args []string
	var err error

	if options.Command != nil {
		cmdString := *options.Command
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

	cmd := exec.Command(packagePath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
