package main

import (
	"encoding/json"
	"fmt"
	"github.com/wellplayedgames/unity-installer/pkg/release"
	"io"
	"os"
)

type distill struct {
	versionSelector
	Output string `short:"o" help:"Output path for spec (defaults to stdout)"`
}

func (d *distill) Run(ctx commandContext) error {
	version, revision, err := d.VersionAndRevision()
	if err != nil {
		return err
	}

	editorRelease, err := ctx.LookupTargetRelease(version, revision)
	if err != nil {
		return err
	}

	spec := &*editorRelease
	selectedModules := map[string]bool{}
	for _, moduleID := range d.Modules {
		selectedModules[moduleID] = true
	}

	modules := make([]release.ModuleRelease, len(spec.Modules))

	for idx := range spec.Modules {
		m := spec.Modules[idx]
		m.Selected = selectedModules[m.ID]
		modules[idx] = m
	}

	spec.Modules = modules

	var output io.Writer = os.Stdout

	if d.Output != "" {
		f, err := os.Create(d.Output)
		if err != nil {
			return fmt.Errorf("failed to create output: %w", err)
		}
		defer func() {
			if err := f.Close(); err != nil {
				ctx.logger.Error(err, "failed to close distil output")
			}
		}()
		output = f
	}

	e := json.NewEncoder(output)
	e.SetIndent("", "  ")

	return e.Encode(spec)
}
