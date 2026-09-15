// Package toolinfo describes the analyzer binary, not the analyzed Go source.
package toolinfo

import (
	"runtime"
	"runtime/debug"
)

type Info struct{ Version, Revision, Modified, GoVersion string }

func Current() Info {
	info, _ := debug.ReadBuildInfo()
	return fromBuildInfo(info)
}
func fromBuildInfo(info *debug.BuildInfo) Info {
	out := Info{Version: "development", GoVersion: runtime.Version()}
	if info == nil {
		return out
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		out.Version = info.Main.Version
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			out.Revision = setting.Value
		case "vcs.modified":
			out.Modified = setting.Value
		}
	}
	return out
}
