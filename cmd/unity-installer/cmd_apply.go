package main

import (
	"encoding/json"
	"fmt"
	"github.com/wellplayedgames/unity-installer/pkg/installer"
	"github.com/wellplayedgames/unity-installer/pkg/release"
	"os"
)

type apply struct {
	Spec    string   `required arg help:"Spec file to apply"`
	Modules []string `name:"module" help:"Extra modules to install whilst applying"`
	Force   bool     `help:"Reinstall Unity"`
}

func (a *apply) Run(ctx commandContext) error {
	spec := &release.EditorRelease{}
	f, err := os.Open(a.Spec)
	if err != nil {
		return fmt.Errorf("failed to open spec: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			ctx.logger.Error(err, "failed to close apply source")
		}
	}()

	d := json.NewDecoder(f)
	err = d.Decode(spec)
	if err != nil {
		return fmt.Errorf("failed to decode spec: %w", err)
	}

	var installModules []string
	for idx := range spec.Modules {
		m := &spec.Modules[idx]
		if m.Selected {
			installModules = append(installModules, m.ID)
		}
	}

	for _, moduleID := range a.Modules {
		installModules = append(installModules, moduleID)
	}

	if !a.Force {
		if has, _ := installer.HasEditorAndModules(ctx.installer, spec.Version, installModules); has {
			return nil
		}
	}

	pkgInstaller := newPackageInstaller(ctx.logger)
	defer func() {
		if err := pkgInstaller.Close(); err != nil {
			ctx.logger.Error(err, "failed to shutdown package installer")
		}
	}()

	return installer.EnsureEditorWithModules(CLI.Platform, ctx.installer, pkgInstaller, spec, installModules, a.Force)
}
