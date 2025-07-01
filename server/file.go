package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func writeFileAtomic(root *os.Root, rootPath string, slug string, v []byte) error {
	const writePermissions = 0o644
	tmpSlug := slug + ".tmp"

	f, err := root.OpenFile(tmpSlug, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, writePermissions)
	if err != nil {
		return fmt.Errorf("failed to open %s for write: %w", tmpSlug, err)
	}

	_, err = f.Write(v)
	if err != nil {
		err = fmt.Errorf("failed to write to %s: %w", tmpSlug, err)
		return errors.Join(err, f.Close())
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("failed to close %s: %w", tmpSlug, err)
	}

	//nolint:godox
	// TODO when we have go 1.25
	// err = s.root.Rename(tmpSlug, slug)
	err = os.Rename(filepath.Join(rootPath, tmpSlug), filepath.Join(rootPath, slug))
	if err != nil {
		return fmt.Errorf("failed to rename %s to %s: %w", tmpSlug, slug, err)
	}

	return nil
}

const defaultFilePermissions = 0o644

func appendContents(root *os.Root, path string, contents []byte) (err error) {
	f, err := root.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, defaultFilePermissions)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer func(f *os.File) {
		closeErr := f.Close()
		if closeErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close %s: %w", path, closeErr))
		}
	}(f)

	_, err = f.Write(contents)
	if err != nil {
		return fmt.Errorf("failed to write to %s: %w", path, err)
	}

	return nil
}
