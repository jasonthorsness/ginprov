package oshelper_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jasonthorsness/ginprov/oshelper"
)

func TestRenameInRoot(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()

	root, err := os.OpenRoot(tmp)
	if err != nil {
		t.Fatalf("os.NewRoot: %v", err)
	}

	oldName := "foo.txt"
	newName := "bar.txt"
	oldPath := filepath.Join(tmp, oldName)

	err = os.WriteFile(oldPath, []byte("hello"), 0o600)
	if err != nil {
		t.Fatalf("write file: %v", err)
	}

	err = oshelper.RenameInRoot(root, oldName, newName)
	if err != nil {
		t.Errorf("renameInRoot failed: %v", err)
	}

	_, err = os.Stat(filepath.Join(tmp, oldName))
	if !os.IsNotExist(err) {
		t.Errorf("old file still exists or unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, newName)) //nolint:gosec
	if err != nil {
		t.Errorf("new file missing: %v", err)
	} else if string(data) != "hello" {
		t.Errorf("new file contents = %q; want %q", data, "hello")
	}
}

func TestRenameInRoot_Errors(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()

	root, err := os.OpenRoot(tmp)
	if err != nil {
		t.Fatalf("os.NewRoot: %v", err)
	}

	tests := []struct {
		name    string
		root    *os.Root
		from    string
		to      string
		wantErr bool
	}{
		{"nil root", nil, "a", "b", true},
		{"empty from", root, "", "b", true},
		{"empty to", root, "a", "", true},
		{"same paths", root, "x", "x", false},
		{"nonexistent", root, "nope", "nope2", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := oshelper.RenameInRoot(tt.root, tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("renameInRoot(root, %q, %q) error = %v, wantErr %v", tt.from, tt.to, err, tt.wantErr)
			}
		})
	}
}
