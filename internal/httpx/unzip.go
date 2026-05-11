package httpx

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

func Ungzip(file *os.File, dstFile string) error {
	file.Seek(0, io.SeekStart)
	archive, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer archive.Close()

	writer, err := os.Create(dstFile)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer writer.Close()
	_, err = io.Copy(writer, archive)
	return err
}
