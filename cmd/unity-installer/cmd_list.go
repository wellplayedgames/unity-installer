package main

import (
	"fmt"
	"sort"
)

type list struct{}

func (l *list) Run(ctx commandContext) error {
	latestReleases, err := ctx.releaseSource.FetchReleases(CLI.Platform, true)
	if err != nil {
		return err
	}

	versions := make([]string, 0, len(latestReleases))
	for version := range latestReleases {
		versions = append(versions, version)
	}

	sort.Strings(versions)

	for _, version := range versions {
		fmt.Println(version)
	}

	return nil
}
