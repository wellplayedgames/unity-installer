// +build !windows

package packageinstaller

func NewDefaultInstaller() (PackageInstaller, error) {
	return &localInstaller{}, nil
}

func MaybeHandleService() {}
