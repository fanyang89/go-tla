package frontend

import (
	"go/token"
	"path/filepath"
	"strings"

	"github.com/fanmi/go-tla/internal/behavior"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

type SourceFile struct{ Path, Package string }

func sourceIndex(ps []*packages.Package) map[string]SourceFile {
	out := map[string]SourceFile{}
	packages.Visit(ps, nil, func(p *packages.Package) {
		for _, file := range p.CompiledGoFiles {
			path := p.PkgPath + "/" + filepath.Base(file)
			if p.Module != nil && p.Module.Main {
				if rel, err := filepath.Rel(p.Module.Dir, file); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					path = filepath.ToSlash(rel)
				}
			}
			out[filepath.Clean(file)] = SourceFile{path, p.PkgPath}
		}
	})
	return out
}

// Position avoids checkout/cache-specific absolute paths. Main-module files are
// module-relative; dependency files are import-qualified. Package remains explicit.
func (p *Program) Position(pos token.Pos, f *ssa.Function) behavior.Position {
	if pos == token.NoPos && f != nil {
		pos = f.Pos()
	}
	raw := p.Fset.Position(pos)
	out := behavior.Position{Line: raw.Line, Column: raw.Column}
	if raw.Filename != "" {
		if file, ok := p.Sources[filepath.Clean(raw.Filename)]; ok {
			out.File, out.Package = file.Path, file.Package
		} else {
			out.File = "unmapped/" + filepath.Base(raw.Filename)
		}
	}
	if f != nil {
		out.Function = f.Name()
		if f.Pkg != nil {
			out.Package = f.Pkg.Pkg.Path()
			out.Function = f.RelString(f.Pkg.Pkg)
		}
	}
	return out
}
