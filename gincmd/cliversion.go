package gincmd

import (
	"fmt"
	"strings"

	version "github.com/hashicorp/go-version"
)

const minAnnexVersion = "6.20171108" // tested working
var minver, _ = version.NewVersion(minAnnexVersion)

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

// GitOK checks if the system git version is higher than the required one.
// If it is not, or the git binary is not found, an appropriate error message is returned.
func (v *VersionInfo) GitOK() (bool, error) {
	_, err := version.NewVersion(v.Git)
	if err != nil {
		return false, fmt.Errorf(v.Git)
	}
	return true, nil
}

// AnnexOK checks if the system annex version is higher than the required one.
// If it is not, or the git-annex binary is not found, an appropriate error message is returned.
func (v *VersionInfo) AnnexOK() (bool, error) {
	compare := func(sv *version.Version) (bool, error) {
		if sv.LessThan(minver) {
			return false, fmt.Errorf("git-annex version %s found, but %s or newer is required", v.Annex, minAnnexVersion)
		}
		return true, nil
	}

	systemver, err := version.NewVersion(v.Annex)
	if err == nil {
		return compare(systemver)
	}

	// Special case for neurodebian git-annex version
	// The version string contains a tilde as a separator for the arch suffix
	// Cutting off the suffix and checking again
	verstring := strings.Split(v.Annex, "~")[0]
	systemver, err = version.NewVersion(verstring)
	if err == nil {
		return compare(systemver)
	}

	// Can't figure out the version: print error from AnnexVersion
	return false, fmt.Errorf(v.Annex)
}
