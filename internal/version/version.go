// Package version reports which build of the code is running, so that stored
// parse results can be traced to the code that produced them.
package version

import "runtime/debug"

// Unknown is reported when the binary carries no version control information,
// for example when built outside a git checkout or with -buildvcs=false.
const Unknown = "unknown"

// Parser returns the short commit hash the binary was built from, with a
// "-dirty" suffix if the working tree had uncommitted changes.
func Parser() string {
	info, _ := debug.ReadBuildInfo()
	return fromBuildInfo(info)
}

func fromBuildInfo(info *debug.BuildInfo) string {
	if info == nil {
		return Unknown
	}
	var revision string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if revision == "" {
		return Unknown
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if modified {
		revision += "-dirty"
	}
	return revision
}
