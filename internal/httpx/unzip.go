package httpx

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Ungzip extracts the gzipped file into dstFile. It extracts to a sibling
// temp file and renames it into place, so a failure never truncates an
// existing (possibly running) binary.
func Ungzip(file *os.File, dstFile string) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek archive: %w", err)
	}
	archive, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer archive.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dstFile), "."+filepath.Base(dstFile)+".tmp*")
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if _, err := io.Copy(tmp, archive); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dstFile); err != nil {
		return err
	}
	tmpName = ""
	return nil
}
