package log

import (
	"os"
	"runtime/debug"
)

// Version is the version tag value to be added to the global logger.
//
// If it's changed to non-empty value before the calling of Init* functions
// (InitFromConfig/InitLogger/InitLoggerJSON/InitLoggerWithConfig/InitSentry),
// the global logger initialized will come with a tag of VersionKey("v"),
// added to every line of logs.
// For InitSentry the global sentry will also be initialized with a tag of
// "version".
//
// In order to use it, either set it in your main function early,
// before calling InitLogger* functions,
// with the value coming from flag/config file/etc..
// For example:
//
//	func main() {
//	  log.Version = *flagVersion
//	  log.InitLoggerJSON(log.Level(*logLevel))
//	  // ...
//	}
//
// Or just "stamp" it during build time,
// by passing additional ldflags to go build command.
// For example:
//
//	go build -ldflags "-X github.com/reddit/baseplate.go/log.Version=$(git rev-parse HEAD)"
//
// Change its value after calling Init* functions will have no effects,
// unless you call Init* functions again.
//
// Starting from go 1.18, if Version is not stamped at compile time,
// we'll try to fill it from runtime/debug.ReadBuildInfo, if available.
var (
	VersionLogKey = "v"

	Version string
)

func init() {
	// Try to read version from build info if it's not stamped at compile time.
	if Version == "" {
		if info, _ := debug.ReadBuildInfo(); info != nil {
			Version = getVersionFromBuildInfo(info)
		}
	}
	// Try to read version from environment variable if it's not stamped at
	// compile time nor from go toolchain
	if Version == "" {
		Version = getVersionFromEnvVar()
	}
}

func getVersionFromBuildInfo(info *debug.BuildInfo) string {
	const (
		versionKey  = "vcs.revision"
		dirtyKey    = "vcs.modified"
		dirtyValue  = "true"
		dirtySuffix = "-dirty"

		untaggedMainVersion = "(devel)"
	)
	var v string
	var dirty bool
	for _, setting := range info.Settings {
		switch setting.Key {
		case versionKey:
			v = setting.Value
		case dirtyKey:
			dirty = setting.Value == dirtyValue
		}
	}
	if v != "" {
		if dirty {
			v = v + dirtySuffix
		}
		return v
	}
	// fallback to the main module version if that's a tagged version
	if info.Main.Version != untaggedMainVersion {
		return info.Main.Version
	}
	return ""
}

const versionEnvVar = "VERSION"

func getVersionFromEnvVar() string {
	return os.Getenv(versionEnvVar)
}
