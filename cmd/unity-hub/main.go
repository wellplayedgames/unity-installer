package main

import (
	"net/http"

	"github.com/wellplayedgames/unity-hub/installer"
	"github.com/wellplayedgames/unity-hub/installer/releases"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	defaultReleasesEndpoint = "https://public-cdn.cloud.unity3d.com/hub/prod/"
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

	cmdInstall         = kingpin.Command("install", "Install a Unity version (optionally with modules)")
	flagInstallVersion = cmdInstall.Arg("version", "Unity version to install.").String()
	flagInstallModules = cmdInstall.Flag("module", "Module to install (can be repeated for multiple modules)").Strings()
)

func main() {
	cmd := kingpin.Parse()

	releaseSource := releases.NewHTTPReleaseSource(http.DefaultClient, *flagReleasesEndpoint)
	releases, err := releaseSource.FetchReleases("win32")
	if err != nil {
		panic(err)
	}

	unityInstaller, err := installer.NewSimpleInstaller(*flagEditorDir, releases, http.DefaultClient)
	if err != nil {
		panic(err)
	}
	defer unityInstaller.Close()

	switch cmd {
	case cmdInstall.FullCommand():
		err = installer.EnsureEditorWithModules(unityInstaller, *flagInstallVersion, *flagInstallModules)
		if err != nil {
			panic(err)
		}
	}
}
