package gincmd

import (
	"fmt"
	"strings"

	version "github.com/hashicorp/go-version"
)

const minAnnexVersion = "6.20160126" // Introduction of git-annex add --json

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
		return "GIN command line client [dev build]"
	}
	return fmt.Sprintf("GIN command line client %s Build %s (%s)\n  git: %s\n  git-annex: %s", v.Version, v.Build, v.Commit, v.Git, v.Annex)
}

// GitOK checks if the system git version is higher than the required one.
// If it is not, or the git binary is not found, an appropriate error message is returned.
func (v *VersionInfo) GitOK() (bool, error) {
	if v.Git == "" {
		return false, fmt.Errorf("The GIN Client requires git to be installed and accessible")
	}
	return true, nil
}

// AnnexOK checks if the system annex version is higher than the required one.
// If it is not, or the git-annex binary is not found, an appropriate error message is returned.
func (v *VersionInfo) AnnexOK() (bool, error) {
	errmsg := fmt.Sprintf("The GIN Client requires git-annex %s or newer", minAnnexVersion)
	systemver, err := version.NewVersion(v.Annex)
	if err != nil {
		// Special case for neurodebian git-annex version
		// The versionn string contains a tilde as a separator for the arch suffix
		// Cutting off the suffix and checking again
		verstring := strings.Split(v.Annex, "~")[0]
		systemver, err = version.NewVersion(verstring)
		if err != nil {
			// Can't figure out the version. Giving up.
			return false, fmt.Errorf("%s\ngit-annex version %s not understood", errmsg, v.Annex)
		}
	}
	minver, _ := version.NewVersion(minAnnexVersion)
	if systemver.LessThan(minver) {
		return false, fmt.Errorf("%s\nFound version %s", errmsg, v.Annex)
	}
	return true, nil
}
