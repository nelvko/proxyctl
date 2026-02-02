package httpx

import (
	"compress/gzip"
	"io"
	"os"
)

func Extract(src io.Reader, dstFile string) error {
	archive, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer archive.Close()

	writer, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer writer.Close()
	_, err = io.Copy(writer, archive)
	return err
}
