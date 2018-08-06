package installer

// EnsureEditorWithModules installs (if missing) an editor version and list of modules.
func EnsureEditorWithModules(installer UnityInstaller, editorVersion string, moduleIDs []string) error {
	hasEditor, existingModules, err := installer.CheckEditorVersion(editorVersion)
	if err != nil {
		return err
	}

	if !hasEditor {
		err = installer.InstallEditor(editorVersion)
		if err != nil {
			return err
		}
	}

	hasModules := map[string]bool{}

	for _, existing := range existingModules {
		hasModules[existing] = true
	}

	for _, moduleID := range moduleIDs {
		if hasModules[moduleID] {
			continue
		}

		err = installer.InstallModule(editorVersion, moduleID)
		if err != nil {
			return err
		}
	}

	return nil
}
