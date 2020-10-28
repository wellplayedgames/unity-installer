package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"gopkg.in/alecthomas/kingpin.v2"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"sort"

	"github.com/go-logr/stdr"
	"github.com/wellplayedgames/unity-installer/pkg/installer"
	pkginstaller "github.com/wellplayedgames/unity-installer/pkg/package-installer"
	"github.com/wellplayedgames/unity-installer/pkg/releases"
)

var (
	flagReleasesEndpoint = kingpin.Flag("releases-endpoint", "Endpoint to fetch Unity releases from.").
				Envar("UNTIY_RELEASES_ENDPOINT").
				String()
	flagEditorDir = kingpin.Flag("install-path", "Directory to install Unity editors to.").
			Default("C:\\Program Files\\Unity").
			Envar("UNITY_INSTALL_PATH").
			String()
	flagPlatform = kingpin.Flag("platform", "Unity host platform").Envar("UNITY_PLATFORM").Default(getPlatform()).String()

	cmdInstall         = kingpin.Command("install", "Install a Unity version (optionally with modules)")
	flagInstallVersion = cmdInstall.Arg("version", "Unity version to install.").Required().String()
	flagInstallModules = cmdInstall.Flag("module", "Module to install (can be repeated for multiple modules)").Strings()

	cmdDistil         = kingpin.Command("distil", "Create an install spec for later.")
	flagDistilVersion = cmdDistil.Arg("version", "Unity version to distil.").Required().String()
	flagDistilModules = cmdDistil.Flag("module", "Module to distil (can be repeated for multiple modules)").Strings()
	flagDistilOutput  = cmdDistil.Flag("output", "Output spec file location.").Short('o').String()

	cmdApply              = kingpin.Command("apply", "Apply a previously distilled install.")
	flagApplyFile         = cmdApply.Arg("spec", "Spec file to apply.").Required().String()
	flagApplyExtraModules = cmdApply.Flag("module", "Extra modules to install whilst applying.").Strings()

	cmdList = kingpin.Command("list", "List available versions.")
)

func getPlatform() string {
	switch runtime.GOOS {
	case "windows":
		return "win32"
	default:
		return runtime.GOOS
	}
}

func getReleases() releases.Releases {
	releaseSource := releases.DefaultReleaseSource

	if *flagReleasesEndpoint != "" {
		releaseSource = releases.NewHTTPReleaseSource(http.DefaultClient, *flagReleasesEndpoint)
	}

	downloadedReleases, err := releaseSource.FetchReleases(*flagPlatform, false)
	if err != nil {
		panic(err)
	}

	return downloadedReleases
}

func lookupTargetRelease(editorVersion string) *releases.EditorRelease {
	downloadedReleases := getReleases()
	editorRelease, ok := downloadedReleases[editorVersion]
	if !ok {
		panic(fmt.Errorf("no such editor version %s", editorVersion))
	}

	return editorRelease
}

func newPackageInstaller(logger logr.Logger) pkginstaller.PackageInstaller {
	pkgInstall, err := pkginstaller.NewDefaultInstaller(logger.WithName("installer"))
	if err != nil {
		panic(err)
	}

	return pkgInstall
}

func main() {
	cmd := kingpin.Parse()
	logger := stdr.New(log.New(os.Stderr, "", log.LstdFlags))
	pkginstaller.MaybeHandleService(logger.WithName("service"))

	tempDir, err := ioutil.TempDir("", "UnityInstaller")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			logger.Error(err, "failed to remove temporary directory")
		}
	}()

	unityInstaller, err := installer.NewSimpleInstaller(logger.WithName("simple-installer"), *flagEditorDir, tempDir, http.DefaultClient)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := unityInstaller.Close(); err != nil {
			logger.Error(err, "failed to shutdown unity installer")
		}
	}()

	switch cmd {
	case cmdInstall.FullCommand():

		has, _ := installer.HasEditorAndModules(unityInstaller, *flagInstallVersion, *flagInstallModules)
		if has {
			return
		}

		pkgInstaller := newPackageInstaller(logger)
		defer func() {
			if err := pkgInstaller.Close(); err != nil {
				logger.Error(err, "failed to shutdown package installer")
			}
		}()

		editorRelease := lookupTargetRelease(*flagInstallVersion)
		err = installer.EnsureEditorWithModules(unityInstaller, pkgInstaller, editorRelease, *flagInstallModules)
		if err != nil {
			panic(err)
		}

	case cmdDistil.FullCommand():
		editorRelease := lookupTargetRelease(*flagDistilVersion)

		spec := &*editorRelease

		selectedModules := map[string]bool{}
		for _, moduleID := range *flagDistilModules {
			selectedModules[moduleID] = true
		}

		modules := make([]releases.ModuleRelease, len(spec.Modules))

		for idx := range spec.Modules {
			m := spec.Modules[idx]
			m.Selected = selectedModules[m.ID]
			modules[idx] = m
		}

		spec.Modules = modules

		var output io.Writer = os.Stdout

		if *flagDistilOutput != "" {
			f, err := os.Create(*flagDistilOutput)
			if err != nil {
				panic(err)
			}
			defer func() {
				if err := f.Close(); err != nil {
					logger.Error(err, "failed to close distil output")
				}
			}()
			output = f
		}

		e := json.NewEncoder(output)
		e.SetIndent("", "  ")

		err := e.Encode(spec)
		if err != nil {
			panic(err)
		}

	case cmdApply.FullCommand():
		spec := &releases.EditorRelease{}
		f, err := os.Open(*flagApplyFile)
		if err != nil {
			panic(err)
		}
		defer func() {
			if err := f.Close(); err != nil {
				logger.Error(err, "failed to close apply source")
			}
		}()

		d := json.NewDecoder(f)
		err = d.Decode(spec)
		if err != nil {
			panic(err)
		}

		var installModules []string
		for idx := range spec.Modules {
			m := &spec.Modules[idx]
			if m.Selected {
				installModules = append(installModules, m.ID)
			}
		}

		for _, moduleID := range *flagApplyExtraModules {
			installModules = append(installModules, moduleID)
		}

		has, _ := installer.HasEditorAndModules(unityInstaller, spec.Version, installModules)
		if has {
			return
		}

		pkgInstaller := newPackageInstaller(logger)
		defer func() {
			if err := pkgInstaller.Close(); err != nil {
				logger.Error(err, "failed to shutdown package installer")
			}
		}()

		err = installer.EnsureEditorWithModules(unityInstaller, pkgInstaller, spec, installModules)
		if err != nil {
			panic(err)
		}

	case cmdList.FullCommand():
		latestReleases := getReleases()

		versions := make([]string, len(latestReleases))

		i := 0
		for version := range latestReleases {
			versions[i] = version
			i++
		}

		sort.Strings(versions)

		for _, version := range versions {
			fmt.Println(version)
		}
	}
}
