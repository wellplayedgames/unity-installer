package main

import (
	"github.com/wellplayedgames/unity-installer/pkg/editor"
	"github.com/wellplayedgames/unity-installer/pkg/installer"
)

type versionSelector struct {
	ForProject string   `help:"Path to Unity project to match version for"`
	Version    string   `help:"Unity version to install"`
	Revision   string   `help:"Unity revision to install"`
	Modules    []string `name:"module" help:"Extra modules to install (can be repeated to specify multiple modules)"`
}

func (s *versionSelector) VersionAndRevision() (string, string, error) {
	if s.ForProject != "" {
		pv, err := editor.ProjectVersionFromProject(s.ForProject)
		if err != nil {
			return "", "", err
		}

		version, revision := pv.VersionAndRevision()
		return version, revision, nil
	}

	return s.Version, s.Revision, nil
}

type install struct {
	versionSelector
	Force bool `help:"Reinstall Unity"`
}

func (i *install) Run(ctx commandContext) error {
	if !i.Force {
		if has, _ := installer.HasEditorAndModules(ctx.installer, i.Version, i.Modules); has {
			return nil
		}
	}

	pkgInstaller := newPackageInstaller(ctx.logger)
	defer func() {
		if err := pkgInstaller.Close(); err != nil {
			ctx.logger.Error(err, "failed to shutdown package installer")
		}
	}()

	version, revision, err := i.VersionAndRevision()
	if err != nil {
		return err
	}

	editorRelease, err := ctx.LookupTargetRelease(version, revision)
	if err != nil {
		return err
	}

	return installer.EnsureEditorWithModules(ctx.installer, pkgInstaller, editorRelease, i.Modules, i.Force)
}
