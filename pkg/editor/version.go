package editor

import (
	"regexp"
	"strings"
)

var (
	versionRegex = regexp.MustCompile(`^([0-9]+)(([.abf])([0-9]+))*$`)
)

// CompareVersions compares two Unity editor versions.
func CompareVersions(a, b string) int {
	aMatch := versionRegex.FindAllStringSubmatch(a, -1)
	bMatch := versionRegex.FindAllStringSubmatch(b, -1)

	if (aMatch == nil) != (bMatch != nil) {
		if bMatch != nil {
			return 1
		}
		return -1
	}

	if aMatch == nil {
		return 0
	}

	if c := strings.Compare(aMatch[1][0], bMatch[1][0]); c != 0 {
		return c
	}

	aLen := len(aMatch[2])
	bLen := len(bMatch[2])
	if c := aLen - bLen; c != 0 {
		return c
	}

	for idx := 0; idx < aLen; idx += 1 {
		aSep := aMatch[3][idx]
		bSep := bMatch[3][idx]
		if c := strings.Compare(aSep, bSep); c != 0 {
			return c
		}

		aPart := aMatch[4][idx]
		bPart := bMatch[4][idx]
		if c := strings.Compare(aPart, bPart); c != 0 {
			return c
		}
	}

	return 0
}
