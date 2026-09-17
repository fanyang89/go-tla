package lowering

import (
	"slices"
	"strconv"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/toolinfo"
)

func modelMetadata(opts Options) behavior.Metadata {
	info := toolinfo.Current()
	trust := slices.Clone(opts.TrustedCalls)
	slices.Sort(trust)
	trust = slices.Compact(trust)
	options := map[string][]string{"trustedCalls": trust, "profile": {"acyclic-static-identity"}}
	if opts.RuntimeProcs != 0 {
		options["startup.GOMAXPROCS"] = []string{strconv.Itoa(opts.RuntimeProcs)}
	}
	return behavior.Metadata{Producer: "gotla", Version: info.Version, Revision: info.Revision, Modified: info.Modified, Language: "Go", Toolchain: info.GoVersion,
		Options: options}
}
