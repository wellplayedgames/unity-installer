package main

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/alecthomas/kong"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/wellplayedgames/unity-installer/pkg/installer"
	pkginstaller "github.com/wellplayedgames/unity-installer/pkg/package-installer"
	"github.com/wellplayedgames/unity-installer/pkg/release"
)

type commandContext struct {
	ctx           context.Context
	logger        logr.Logger
	releaseSource release.Source
	installer     installer.UnityInstaller
}

var CLI struct {
	ReleasesEndpoint string `help:"Endpoint to fetch Unity releases from" env:"UNITY_RELEASES_ENDPOINT"`
	ArchiveEndpoint  string `help:"Endpoint to fetch archived Unity releases from" env:"UNITY_ARCHIVE_ENDPOINT"`

	InstallPath string `help:"Directory to install Unity editors into" env:"UNITY_INSTALL_PATH" default:"C:\\Program Files\\Unity"`
	Platform    string `help:"Unity host platform" env:"UNITY_PLATFORM" default:"${default_platform}"`

	Install install `cmd help:"Install a Unity version (optionally with modules)"`
	Distill distill `cmd help:"Create an install spec to install later"`
	Apply   apply   `cmd help:"Apply a previously distilled install spec"`
	List    list    `cmd help:"List available Unity versions"`
}

func getPlatform() string {
	switch runtime.GOOS {
	case "windows":
		return "win32"
	default:
		return runtime.GOOS
	}
}

func getReleaseSource() release.Source {
	releaseSource := release.DefaultReleaseSource

	if CLI.ReleasesEndpoint != "" {
		releaseSource.PublishedVersionsEndpoint = CLI.ReleasesEndpoint
	}

	if CLI.ArchiveEndpoint != "" {
		releaseSource.GAArchiveURL = CLI.ArchiveEndpoint
		releaseSource.TestingArchiveURL = CLI.ArchiveEndpoint
	}

	return release.NewCache(&releaseSource)
}

func (c commandContext) LookupTargetRelease(version, revision string) (*release.EditorRelease, error) {
	return c.releaseSource.FetchRelease(CLI.Platform, version, revision)
}

func newPackageInstaller(logger logr.Logger) pkginstaller.PackageInstaller {
	pkgInstall, err := pkginstaller.NewDefaultInstaller(logger.WithName("installer"))
	if err != nil {
		panic(err)
	}

	return pkgInstall
}

func main() {
	logger := stdr.New(log.New(os.Stderr, "", log.LstdFlags))
	pkginstaller.MaybeHandleService(logger.WithName("service"))

	args := kong.Parse(&CLI, kong.Vars{
		"default_platform": getPlatform(),
	})

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		cancelCtx()
		<-signals
		logger.Info("Double ctrl-c, exiting immediately")
		os.Exit(1)
	}()

	tempDir, err := ioutil.TempDir("", "UnityInstaller")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			logger.Error(err, "failed to remove temporary directory")
		}
	}()

	unityInstaller, err := installer.NewSimpleInstaller(logger.WithName("simple-installer"), CLI.InstallPath, tempDir, http.DefaultClient)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := unityInstaller.Close(); err != nil {
			logger.Error(err, "failed to shutdown unity installer")
		}
	}()

	cmdCtx := commandContext{
		ctx:           ctx,
		logger:        logger,
		releaseSource: getReleaseSource(),
		installer:     unityInstaller,
	}
	if err := args.Run(cmdCtx); err != nil {
		logger.Error(err, "failed to run command")
		os.Exit(1)
	}
}
