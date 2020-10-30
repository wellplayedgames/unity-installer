// +build !windows

package packageinstaller

func NewDefaultInstaller(logger logr.Logger) (PackageInstaller, error) {
	return NewLocalInstaller(logger), nil
}

func MaybeHandleService() {}
