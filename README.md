# Unity Installer
Go implementation of Unity Hub's downloader and installer - designed for use in continuous integration.

# Installing
1. Download `unity-installer`
    - Windows: [unity-installer.exe](https://github.com/wellplayedgames/unity-installer/releases/latest/download/unity-installer.exe)
    - Linux: [unity-installer-linux-amd64](https://github.com/wellplayedgames/unity-installer/releases/latest/download/unity-installer-linux-amd64)
    - macOS: [unity-installer-darwin-amd64](https://github.com/wellplayedgames/unity-installer/releases/latest/download/unity-installer-darwin-amd64)
2. Rename this to unity-installer if not on Windows and put it in the client Unity project
3. This may need making executable using `chmod +x unity-installer` 

# Building
This requires to be built for all platforms. In `cmd/unity-installer` on windows:
```
# Windows
go build .

# Linux
env GOOS=linux GOARCH=amd64 go build .
mv unity-installer unity-installer-linux-amd64

# Darwin (mac)
env GOOS=darwin GOARCH=amd64 go build .
mv unity-installer unity-installer-darwin-amd64
```

# Usage

## Install version set in project
Example setup for a project running on windows setup.
* Tools installed in platform folders in the `Scripts` folder at the root:
  * `Scripts/Darwin/unity-installer`
  * `Scripts/Linux/unity-installer`
  * `Scripts/Windows/unity-installer.exe`
* In the root directory:
  ```
  ./Scripts/Windows/unity-installer.exe --install-path="C:\Unity" install --for-project=Client
  ```
  **NOTE:** The install path for unity should be set the same in UnityHub installs can be shared
