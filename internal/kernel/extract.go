package kernel

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func extractArtifact(file *os.File, dst string, art *Artifact) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	switch art.Archive {
	case "gz":
		return ungzip(file, dst)
	case "tar.gz":
		if art.Entry == "" {
			return errors.New("tar.gz release has no executable entry")
		}
		gz, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			h, err := tr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
			if h.Name == art.Entry {
				if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
					return errors.New("archive entry is not a regular file")
				}
				return writeExecutable(dst, io.LimitReader(tr, h.Size))
			}
		}
		return fmt.Errorf("archive has no entry %q", art.Entry)
	default:
		return fmt.Errorf("unsupported archive format %q", art.Archive)
	}
}

func extractExtraFiles(file *os.File, bin string, art *Artifact) error {
	if len(art.Extra) == 0 {
		return nil
	}
	if art.Archive != "tar.gz" {
		return errors.New("companion files require tar.gz")
	}
	for _, entry := range art.Extra {
		if filepath.Base(entry) == "." || filepath.Base(entry) == ".." {
			return fmt.Errorf("invalid companion %q", entry)
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		gz, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		tr := tar.NewReader(gz)
		found := false
		for {
			h, err := tr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				gz.Close()
				return err
			}
			if h.Name != entry {
				continue
			}
			if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
				gz.Close()
				return fmt.Errorf("companion %q is not a regular file", entry)
			}
			if err := writeStagedFile(filepath.Join(filepath.Dir(bin), filepath.Base(entry)), io.LimitReader(tr, h.Size), 0o644); err != nil {
				gz.Close()
				return err
			}
			found = true
			break
		}
		gz.Close()
		if !found {
			return fmt.Errorf("archive has no companion %q", entry)
		}
	}
	return nil
}

func writeExecutable(dst string, src io.Reader) error {
	return writeStagedFile(dst, src, 0o755)
}

func writeStagedFile(dst string, src io.Reader, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := io.Copy(f, src); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), dst)
}
