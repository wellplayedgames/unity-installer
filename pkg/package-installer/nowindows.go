// +build !windows

package packageinstaller

import (
	"github.com/go-logr/logr"
)

func NewDefaultInstaller(logger logr.Logger, dryRun bool) (PackageInstaller, error) {
	return NewLocalInstaller(logger, dryRun), nil
}

func MaybeHandleService(logger logr.Logger) {}
