// Package artifact owns current-run output files and serializes writers per directory.
package artifact

import (
	"fmt"
	"os"
	"path/filepath"
)

var managedFiles = [...]string{"result.json", "model.tla", "model.cfg", "model.json", "tlc.log"}

type Directory struct {
	Path string
	lock string
}

// Begin invalidates previous managed artifacts before analysis, including on a
// subsequent package-load failure. Unrelated files and directories are preserved.
func Begin(path string) (*Directory, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return nil, err
	}
	d := &Directory{Path: abs, lock: filepath.Join(abs, ".gotla.lock")}
	if err := os.Mkdir(d.lock, 0700); err != nil {
		return nil, fmt.Errorf("cannot acquire output lock %s (another writer or stale lock; verify no writer is active before removing it): %w", d.lock, err)
	}
	for _, name := range managedFiles {
		if err := os.Remove(filepath.Join(abs, name)); err != nil && !os.IsNotExist(err) {
			d.Close()
			return nil, fmt.Errorf("remove stale %s: %w", name, err)
		}
	}
	return d, nil
}

func (d *Directory) Close() error { return os.Remove(d.lock) }

// Write uses same-directory rename, so a reader never sees truncated JSON or TLA.
// A complete check is identified by result.json status, not by file existence.
func (d *Directory) Write(name string, data []byte) error {
	allowed := false
	for _, file := range managedFiles {
		if name == file {
			allowed = true
		}
	}
	if !allowed {
		return fmt.Errorf("unmanaged artifact name %q", name)
	}
	f, err := os.CreateTemp(d.Path, ".gotla-write-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(d.Path, name))
}

func (d *Directory) RemoveExecutable() {
	// Best effort on an output failure: the caller must still report that failure.
	for _, name := range []string{"model.tla", "model.cfg"} {
		os.Remove(filepath.Join(d.Path, name))
	}
}
