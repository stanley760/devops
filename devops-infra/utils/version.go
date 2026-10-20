package utils

import (
	"os"
	"runtime/debug"
)

// resolveVersion gets the service version by priority:
//  1. APP_VERSION from the .env file (local go run development with an explicit version)
//  2. Module version (when installed via go install from the module proxy)
//  3. vcs.revision (git commit short hash + dirty marker)
//  4. Fallback: unknown
func ResolveVersion() string {

	if v := os.Getenv("APP_VERSION"); v != "" {
		return v
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}

		var revision, modified string
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				revision = s.Value
			case "vcs.modified":
				if s.Value == "true" {
					modified = "_dirty"
				}
			}
		}
		if revision != "" {
			if len(revision) > 7 {
				revision = revision[:7]
			}
			return revision + modified
		}
	}

	return "unknown"
}
