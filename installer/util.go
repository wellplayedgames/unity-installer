package installer

import (
	"fmt"

	packageinstaller "github.com/wellplayedgames/unity-installer/package-installer"
	"github.com/wellplayedgames/unity-installer/releases"
)

// HasEditorAndModules returns true if the given editor version and all given module IDs are installed.
func HasEditorAndModules(installer UnityInstaller, editorVersion string, moduleIDs []string) (bool, error) {
	hasEditor, existingModules, err := installer.CheckEditorVersion(editorVersion)
	if !hasEditor || err != nil {
		return false, err
	}

	existingModMap := map[string]*releases.ModuleRelease{}

	for idx := range existingModules {
		m := &existingModules[idx]
		existingModMap[m.ID] = m
	}

	for _, moduleID := range moduleIDs {
		m, ok := existingModMap[moduleID]
		if !ok || !m.Selected {
			return false, nil
		}
	}

	return true, nil
}

// EnsureEditorWithModules installs (if missing) an editor version and list of modules.
func EnsureEditorWithModules(
	unityInstaller UnityInstaller,
	packageInstaller packageinstaller.PackageInstaller,
	editorRelease *releases.EditorRelease,
	moduleIDs []string) error {

	hasEditor, existingModules, err := unityInstaller.CheckEditorVersion(editorRelease.Version)
	if err != nil {
		return err
	}

	if !hasEditor {
		err = unityInstaller.InstallEditor(packageInstaller, editorRelease)
		if err != nil {
			return err
		}
	}

	existingModSet := map[string]bool{}

	for idx := range existingModules {
		m := &existingModules[idx]
		if m.Selected {
			existingModSet[m.ID] = true
		}
	}

	availableModMap := map[string]*releases.ModuleRelease{}

	for idx := range editorRelease.Modules {
		m := &editorRelease.Modules[idx]
		availableModMap[m.ID] = m
	}

	for _, moduleID := range moduleIDs {
		if existingModSet[moduleID] {
			continue
		}

		m, ok := availableModMap[moduleID]

		if !ok {
			return fmt.Errorf("Missing module %s", moduleID)
		}

		err = unityInstaller.InstallModule(packageInstaller, editorRelease.Version, m)
		if err != nil {
			return err
		}

		existingModSet[moduleID] = true
	}

	return nil
}
