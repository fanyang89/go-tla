package lowering

import (
	"slices"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/toolinfo"
)

func modelMetadata(opts Options) behavior.Metadata {
	info := toolinfo.Current()
	trust := slices.Clone(opts.TrustedCalls)
	slices.Sort(trust)
	trust = slices.Compact(trust)
	return behavior.Metadata{Producer: "gotla", Version: info.Version, Revision: info.Revision, Modified: info.Modified, Language: "Go", Toolchain: info.GoVersion,
		Options: map[string][]string{"trustedCalls": trust, "profile": {"acyclic-static-identity"}}}
}
