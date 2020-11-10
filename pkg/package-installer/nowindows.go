// +build !windows

package packageinstaller

import (
	"github.com/go-logr/logr"
)

func NewDefaultInstaller(logger logr.Logger) (PackageInstaller, error) {
	return NewLocalInstaller(logger), nil
}

func MaybeHandleService(logger logr.Logger) {}
