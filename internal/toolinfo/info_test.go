package toolinfo

import (
	"runtime/debug"
	"testing"
)

func TestBuildProvenance(t *testing.T) {
	if got := fromBuildInfo(nil); got.Version != "development" || got.GoVersion == "" {
		t.Fatalf("missing development fallback: %+v", got)
	}
	info := &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}, Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}, {Key: "vcs.modified", Value: "true"}}}
	got := fromBuildInfo(info)
	if got.Version != "v0.1.0" || got.Revision != "abc" || got.Modified != "true" {
		t.Fatalf("missing binary provenance: %+v", got)
	}
}
