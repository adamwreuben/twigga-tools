package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ComputeReleaseHash(baseDir string, files []string) (string, error) {
	h := sha256.New()
	for _, full := range files {
		// get relative path and normalize slashes
		rel := strings.TrimPrefix(full, baseDir)
		rel = strings.TrimPrefix(rel, string(os.PathSeparator))
		rel = filepath.ToSlash(rel)

		// write relative path and a null byte
		h.Write([]byte(rel))
		h.Write([]byte{0})

		// open file and copy contents into hash
		f, err := os.Open(full)
		if err != nil {
			return "", err
		}
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return "", err
		}

		// write a null byte to separate files
		h.Write([]byte{0})
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)[:12], nil
}
