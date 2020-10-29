package release

import (
	"fmt"
	"github.com/wellplayedgames/unity-installer/pkg/editor"
	"path"
	"strings"
)

func stringPtr(s string) *string {
	return &s
}

func editorDataPath(platform, s string) *string {
	prefix := "{UNITY_PATH}/Editor/Data/"
	if platform == "darwin" {
		prefix = "{UNITY_PATH}"
	}

	out := prefix
	if out != "" {
		out = fmt.Sprintf("%s%s", prefix, s)
	}

	return stringPtr(out)
}

func moduleDestination(platform, name, ext string) *string {
	switch name {
	case "mono":
		fallthrough
	case "visualstudio":
		return nil
	case "monodevelop":
		return stringPtr("{UNITY_PATH}")
	case "documentation":
		if ext == "zip" {
			return editorDataPath(platform, "")
		}
		return stringPtr("{UNITY_PATH}")
	case "standardassets":
		if platform == "darwin" {
			return stringPtr("{UNITY_PATH}/Standard Assets")
		}

		return stringPtr("{UNITY_PATH}/Editor")
	case "exampleprojects":
		fallthrough
	case "example":
		return nil
	case "android":
		return editorDataPath(platform, "PlaybackEngines/AndroidPlayer")
	case "android-sdk-build-tools":
		if ext == "zip" {
			return editorDataPath(platform, "PlaybackEngines/AndroidPlayer/SDK/build-tools")
		}
		return editorDataPath(platform, "PlaybackEngines/AndroidPlayer/SDK")
	case "android-sdk-platforms":
		if ext == "zip" {
			return editorDataPath(platform, "PlaybackEngines/AndroidPlayer/SDK/platforms")
		}
		return editorDataPath(platform, "PlaybackEngines/AndroidPlayer/SDK")
	case "android-sdk-platform-tools":
		fallthrough
	case "android-sdk-ndk-tools":
		return editorDataPath(platform, "PlaybackEngines/AndroidPlayer/SDK")
	case "android-ndk":
		return editorDataPath(platform, "PlaybackEngines/AndroidPlayer/NDK")
	case "android-open-jdk":
		return editorDataPath(platform, "PlaybackEngines/AndroidPlayer/OpenJDK")
	case "ios":
		return stringPtr("{UNITY_PATH}/PlaybackEngines")
	case "tvos":
		fallthrough
	case "appletv":
		return editorDataPath(platform, "PlaybackEngines/AppleTVSupport")
	case "linux":
		fallthrough
	case "linux-mono":
		fallthrough
	case "linux-il2cpp":
		return editorDataPath(platform, "PlaybackEngines/LinuxStandaloneSupport")
	case "mac":
		fallthrough
	case "mac-mono":
		fallthrough
	case "mac-il2cpp":
		return stringPtr("{UNITY_PATH}/Unity.app/Contents/PlaybackEngines/MacStandaloneSupport")
	case "samsungtv":
		fallthrough
	case "samsung-tv":
		return editorDataPath(platform, "PlaybackEngines/STVPlayer")
	case "tizen":
		return editorDataPath(platform, "PlaybackEngines/TizenPlayer")
	case "vuforia":
		fallthrough
	case "vuforia-ar":
		return editorDataPath(platform, "PlaybackEngines/VuforiaSupport")
	case "webgl":
		return editorDataPath(platform, "PlaybackEngines/WebGLSupport")
	case "windows":
		fallthrough
	case "windows-mono":
		fallthrough
	case "windows-il2cpp":
		return editorDataPath(platform, "PlaybackEngines/WindowsStandaloneSupport")
	case "facebook":
		fallthrough
	case "facebook-games":
		return editorDataPath(platform, "PlaybackEngines/Facebook")
	case "facebookgameroom":
		return nil
	case "lumin":
		return editorDataPath(platform, "PlaybackEngines/LuminSupport")
	}

	if strings.HasPrefix(name, "language-") {
		if platform == "darwin" {
			return stringPtr("{UNITY_PATH}/Unity.app/Contents/Localization")
		}
		return editorDataPath(platform, "Localization")
	}

	return stringPtr("{UNITY_PATH}")
}

func prepareExternalModule(platform string, m *ModuleRelease) {
	ext := path.Ext(m.DownloadURL)
	m.InstallOptions.Destination = moduleDestination(platform, m.ID, ext)
}

func generateAndroidModules(release *EditorRelease, platform string) {
	editorVersion := release.Version
	arch := "x86_64"
	host := platform
	if platform == "win32" {
		host = "windows"
	}

	var modules []ModuleRelease

	// Add Android platform.
	if editor.CompareVersions(editorVersion, "2019.4.9f1") > 0 {
		modules = append(modules, ModuleRelease{
			ID:   "android-sdk-platforms",
			Name: "Android SDK Platforms 28",
			Package: Package{
				InstallOptions: InstallOptions{
					RenameFrom: editorDataPath(platform, "PlaybackEngines/AndroidPlayer/SDK/platforms/android-10"),
					RenameTo:   editorDataPath(platform, "PlaybackEngines/AndroidPlayer/SDK/platforms/android-29"),
				},
				DownloadURL: "https://dl.google.com/android/repository/platform-29_r05.zip",
				Version:     "29",
			},
		})
	} else {
		modules = append(modules, ModuleRelease{
			ID:   "android-sdk-platforms",
			Name: "Android SDK Platforms 28",
			Package: Package{
				InstallOptions: InstallOptions{
					RenameFrom: editorDataPath(platform, "PlaybackEngines/AndroidPlayer/SDK/platforms/android-9"),
					RenameTo:   editorDataPath(platform, "PlaybackEngines/AndroidPlayer/SDK/platforms/android-28"),
				},
				DownloadURL: "https://dl.google.com/android/repository/platform-28_r06.zip",
				Version:     "28",
			},
		})
	}

	// Add host tools.
	modules = append(modules, ModuleRelease{
		ID:   "android-sdk-ndk-tools",
		Name: "Android SDK & NDK Tools",
		Package: Package{
			DownloadURL: fmt.Sprintf("https://dl.google.com/android/repository/sdk-tools-%s-4333796.zip", host),
			Version:     "26.1.1",
		},
	}, ModuleRelease{
		ID:   "android-sdk-platform-tools",
		Name: "Android SDK Platform Tools",
		Package: Package{
			DownloadURL: fmt.Sprintf("https://dl.google.com/android/repository/platform-tools_r28.0.1-%s.zip", host),
			Version:     "28.0.1",
		},
	}, ModuleRelease{
		ID:   "android-sdk-build-tools",
		Name: "Android SDK Build Tools",
		Package: Package{
			DownloadURL: fmt.Sprintf("https://dl.google.com/android/repository/build-tools_r28.0.3-%s.zip", host),
			Version:     "28.0.3",
		},
	})

	// Add NDK.
	ndkVersion := "16b"
	if editor.CompareVersions(editorVersion, "20201.1.0a1") > 0 {
		ndkVersion = "21d"
	} else if editor.CompareVersions(editorVersion, "2019.3.0a4") > 0 {
		ndkVersion = "19"
	}
	modules = append(modules, ModuleRelease{
		ID:   "android-ndk",
		Name: fmt.Sprint("Android NDK %s", ndkVersion),
		Package: Package{
			InstallOptions: InstallOptions{
				RenameFrom: editorDataPath(platform, fmt.Sprintf("PlaybackEngines/AndroidPlayer/NDK/android-ndk-r%s", ndkVersion)),
				RenameTo:   editorDataPath(platform, "PlaybackEngines/AndroidPlayer/NDK"),
			},
			DownloadURL: fmt.Sprintf("https://dl.google.com/android/repository/android-ndk-r%s-%s-%s.zip", ndkVersion, host, arch),
			Version:     ndkVersion,
		},
	})

	// Add OpenJDK.
	downloadURL := ""
	switch host {
	case "windows":
		downloadURL = "http://download.unity3d.com/download_unity/open-jdk/open-jdk-win-x64/jdk8u172-b11_4be8440cc514099cfe1b50cbc74128f6955cd90fd5afe15ea7be60f832de67b4.zip"
	case "darwin":
		downloadURL = "http://download.unity3d.com/download_unity/open-jdk/open-jdk-mac-x64/jdk8u172-b11_4be8440cc514099cfe1b50cbc74128f6955cd90fd5afe15ea7be60f832de67b4.zip"
	case "linux":
		downloadURL = "http://download.unity3d.com/download_unity/open-jdk/open-jdk-linux-x64/jdk8u172-b11_4be8440cc514099cfe1b50cbc74128f6955cd90fd5afe15ea7be60f832de67b4.zip"
	}
	modules = append(modules, ModuleRelease{
		ID:   "android-open-jdk",
		Name: "OpenJDK",
		Package: Package{
			DownloadURL: downloadURL,
			Version:     "8u172-b11",
		},
	})

	for idx := range modules {
		mod := modules[idx]
		prepareExternalModule(platform, &mod)
		release.Modules = append(release.Modules, mod)
	}
}
