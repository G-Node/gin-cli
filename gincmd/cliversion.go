package gincmd

import (
	"fmt"
	"strconv"
	"strings"
)

const minAnnexVersion = "6.20171108" // tested working

// VersionInfo holds the version numbers supplied by the linker flags in a convenient struct.
type VersionInfo struct {
	Version string
	Build   string
	Commit  string
	Git     string
	Annex   string
}

// String constructs a human-readable string that contains the version numbers.
func (v *VersionInfo) String() string {
	if v.Version == "" {
		v.Version = "[dev build]"
		v.Build = "[dev]"
		v.Commit = "???"
	}

	gitver := v.Git
	if ok, err := v.GitOK(); !ok {
		if strings.Contains(err.Error(), "not found") {
			gitver = "not found"
		}
	}

	annexver := v.Annex
	if ok, err := v.AnnexOK(); !ok {
		annexver = err.Error()
		if strings.Contains(err.Error(), "not found") {
			annexver = "not found"
		}
	}

	return fmt.Sprintf("GIN command line client %s Build %s (%s)\n  git: %s\n  git-annex: %s", v.Version, v.Build, v.Commit, gitver, annexver)
}

// GitOK checks if git runs and returns an understandable version string.
func (v *VersionInfo) GitOK() (bool, error) {
	_, err := parsever(v.Git)
	if err != nil {
		return false, fmt.Errorf(v.Git)
	}
	return true, nil
}

// AnnexOK checks if the system annex version is higher than the required one.
// If it is not, or the git-annex binary is not found, an appropriate error message is returned.
func (v *VersionInfo) AnnexOK() (bool, error) {
	systemver, err := parsever(v.Annex)
	if err != nil {
		return false, err
	}
	minver, _ := parsever(minAnnexVersion)

	errmsg := fmt.Errorf("git-annex version %s found, but %s or newer is required", v.Annex, minAnnexVersion)

	for idx := range minver {
		if idx >= len(systemver) || systemver[idx] < minver[idx] {
			// if we run out of components for systemver, assume 0 => not newer
			return false, errmsg
		}
		if systemver[idx] > minver[idx] {
			return true, nil
		}
	}

	// all components equal: OK!
	return true, nil
}

// parsever is a very lax version parser that simply splits a version string on '.', '-', and '~' and returns the components in an integer slice.
// The parsing stops when it finds anything that doesn't look like an integer.
// An error is returned only if the first component is not a number.
func parsever(v string) ([]int, error) {

	delims := []rune{'.', '-', '~'}
	f := func(r rune) bool {
		for _, s := range delims {
			if r == s {
				return true
			}
		}
		return false
	}
	components := strings.FieldsFunc(v, f)
	verints := make([]int, 0, len(components))

	for idx, comp := range components {
		vi, err := strconv.Atoi(comp)
		if err != nil {
			if idx == 0 {
				return nil, fmt.Errorf("%s: version string not understood", v)
			}
			break
		}
		verints = append(verints, vi)
	}

	return verints, nil
}
