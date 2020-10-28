package main

import (
	"encoding/json"
	"fmt"
	"gopkg.in/alecthomas/kingpin.v2"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"sort"

	"github.com/wellplayedgames/unity-installer/pkg/installer"
	pkginstaller "github.com/wellplayedgames/unity-installer/pkg/package-installer"
	"github.com/wellplayedgames/unity-installer/pkg/releases"
)

const (
	defaultReleasesEndpoint = "https://public-cdn.cloud.unity3d.com/hub/prod/"
	globalMutexName         = "Global\\UnityInstaller"
)

var (
	flagReleasesEndpoint = kingpin.Flag("releases-endpoint", "Endpoint to fetch Unity releases from.").
				Default(defaultReleasesEndpoint).
				OverrideDefaultFromEnvar("UNTIY_RELEASES_ENDPOINT").
				String()
	flagEditorDir = kingpin.Flag("install-path", "Directory to install Unity editors to.").
			Default("C:\\Program Files\\Unity").
			OverrideDefaultFromEnvar("UNITY_INSTALL_PATH").
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
	releaseSource := releases.NewHTTPReleaseSource(http.DefaultClient, *flagReleasesEndpoint)
	releases, err := releaseSource.FetchReleases(*flagPlatform, false)
	if err != nil {
		panic(err)
	}

	return releases
}

func lookupTargetRelease(editorVersion string) *releases.EditorRelease {
	releases := getReleases()
	editorRelease, ok := releases[editorVersion]
	if !ok {
		panic(fmt.Errorf("no such editor version %s", editorVersion))
	}

	return editorRelease
}

func newPackageInstaller() pkginstaller.PackageInstaller {
	pkgInstall, err := pkginstaller.NewDefaultInstaller()
	if err != nil {
		panic(err)
	}

	return pkgInstall
}

func main() {
	cmd := kingpin.Parse()

	pkginstaller.MaybeHandleService()

	tempDir, err := ioutil.TempDir("", "UnityInstaller")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempDir)

	unityInstaller, err := installer.NewSimpleInstaller(*flagEditorDir, tempDir, http.DefaultClient)
	if err != nil {
		panic(err)
	}
	defer unityInstaller.Close()

	switch cmd {
	case cmdInstall.FullCommand():

		has, _ := installer.HasEditorAndModules(unityInstaller, *flagInstallVersion, *flagInstallModules)
		if has {
			return
		}

		pkgInstaller := newPackageInstaller()
		defer pkgInstaller.Close()

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
			defer f.Close()
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
		defer f.Close()

		d := json.NewDecoder(f)
		err = d.Decode(spec)
		if err != nil {
			panic(err)
		}

		installModules := []string{}
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

		pkgInstaller := newPackageInstaller()
		defer pkgInstaller.Close()

		err = installer.EnsureEditorWithModules(unityInstaller, pkgInstaller, spec, installModules)
		if err != nil {
			panic(err)
		}

	case cmdList.FullCommand():
		releases := getReleases()

		versions := make([]string, len(releases))

		i := 0
		for version := range releases {
			versions[i] = version
			i++
		}

		sort.Strings(versions)

		for _, version := range versions {
			fmt.Println(version)
		}
	}
}
