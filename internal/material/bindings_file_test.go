package material

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

// Catches fallback on broken explicit config, unbounded/device reads and path leaks.
func TestLoadCatalogBindingsFile(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "bindings.json")
	if err := os.WriteFile(good, []byte("["+bindingJSON+"]"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCatalogBindingsFile(good)
	if err != nil || !reflect.DeepEqual(got, []CatalogBinding{{"JD", "PRODUCT", "H5", "home", "test-media"}}) {
		t.Fatal(got, err)
	}
	got, err = LoadCatalogBindingsFile("")
	if err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	empty := filepath.Join(dir, "empty.json")
	if err = os.WriteFile(empty, []byte("[]"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = LoadCatalogBindingsFile(empty)
	if err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	bad := filepath.Join(dir, "private-config.json")
	if err = os.WriteFile(bad, []byte(`{"secret":"private-secret"}`), 0600); err != nil {
		t.Fatal(err)
	}
	large := filepath.Join(dir, "large.json")
	if err = os.WriteFile(large, []byte("[]"+strings.Repeat(" ", 65535)), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err = os.Symlink(good, link); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "pipe")
	if err = syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{bad, large, link, fifo, dir, filepath.Join(dir, "missing"), "relative.json", " "} {
		t.Run(filepath.Base(name), func(t *testing.T) {
			got, err := LoadCatalogBindingsFile(name)
			if got != nil || !errors.Is(err, ErrInvalid) {
				t.Fatal(got, err)
			}
			for _, private := range []string{dir, "private-secret", "test-media"} {
				if strings.Contains(err.Error(), private) {
					t.Fatal("config leak", err)
				}
			}
		})
	}
}
