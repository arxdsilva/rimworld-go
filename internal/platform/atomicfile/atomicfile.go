// Package atomicfile writes a file so that a crash or a failed write leaves
// either the old contents or the new ones, never a mix.
package atomicfile

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Write creates path through write, making its folder if needed. The data goes
// to a temporary file in the same folder, is flushed to disk, and only then
// replaces path. If anything fails, path is untouched and the temporary file is
// gone. perm is the file's final mode, set exactly: the umask doesn't apply.
func Write(path string, perm fs.FileMode, write func(io.Writer) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	if err := fill(tmp, write); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Chmod(tmp.Name(), perm); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	syncDir(dir)
	return nil
}

// WriteBytes is Write for data already in memory.
func WriteBytes(path string, perm fs.FileMode, data []byte) error {
	return Write(path, perm, func(w io.Writer) error {
		_, err := io.Copy(w, bytes.NewReader(data))
		return err
	})
}

// Remove deletes path and flushes its folder, so the removal survives a
// crash. A missing path is os.Remove's error (errors.Is fs.ErrNotExist).
func Remove(path string) error {
	if err := os.Remove(path); err != nil {
		return err
	}
	syncDir(filepath.Dir(path))
	return nil
}

func fill(f *os.File, write func(io.Writer) error) error {
	if err := write(f); err != nil {
		return err
	}
	return f.Sync()
}

// syncDir flushes the folder's entries (the rename) to disk. Best effort: some
// systems cannot sync a directory, and the file is complete anyway.
func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
}
