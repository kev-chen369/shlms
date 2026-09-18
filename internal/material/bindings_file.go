package material

import (
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// LoadCatalogBindingsFile reads trusted Unix deployment configuration once.
// Empty selects the closed default; explicit invalid files never fall back.
// The containing directories must be controlled by the deployment operator.
func LoadCatalogBindingsFile(path string) ([]CatalogBinding, error) {
	if path == "" {
		return nil, nil
	}
	if !filepath.IsAbs(path) {
		return nil, ErrInvalid
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() > CatalogBindingsMaxBytes {
		return nil, ErrInvalid
	}
	// NONBLOCK prevents a replacement FIFO/device from hanging startup.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, ErrInvalid
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) || after.Size() > CatalogBindingsMaxBytes {
		return nil, ErrInvalid
	}
	data, err := io.ReadAll(io.LimitReader(f, CatalogBindingsMaxBytes+1))
	if err != nil {
		return nil, ErrInvalid
	}
	return ParseCatalogBindings(data)
}
