package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/fanmi/go-tla/internal/artifact"
	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/tla"
)

func printDiagnostics(w io.Writer, m *behavior.Model) {
	for _, d := range m.Diagnostics {
		fmt.Fprintf(w, "%s: %s:%d [%s] %s\n", d.Severity, d.File, d.Line, d.Code, d.Message)
	}
}

func writeJSON(dir *artifact.Directory, name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return dir.Write(name, append(data, '\n'))
}

func emitModel(dir *artifact.Directory, m *behavior.Model) (spec, cfg string, err error) {
	if err = writeJSON(dir, "model.json", m); err != nil || m.HasErrors() {
		return "", "", err
	}
	spec, cfg, err = tla.Generate(m)
	if err != nil {
		return "", "", err
	}
	for _, f := range []struct{ name, content string }{{"model.tla", spec}, {"model.cfg", cfg}} {
		if err = dir.Write(f.name, []byte(f.content)); err != nil {
			dir.RemoveExecutable()
			return "", "", err
		}
	}
	return spec, cfg, nil
}
