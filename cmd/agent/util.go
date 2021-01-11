package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
)

func readAndCompressFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := new(bytes.Buffer)
	gzip := gzip.NewWriter(buf)
	io.Copy(gzip, f)
	return buf.Bytes(), nil
}
