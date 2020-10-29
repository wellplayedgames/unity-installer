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

	versions := make([]string, len(latestReleases))
	i := 0
	for version := range latestReleases {
		versions[i] = version
		i++
	}

	sort.Strings(versions)

	for _, version := range versions {
		fmt.Println(version)
	}

	return nil
}
