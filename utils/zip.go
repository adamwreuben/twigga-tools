package utils

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

// ZipDirectoryExcluding zips a source directory but ignores specified folders (like node_modules)
func ZipDirectoryExcluding(sourceDir, destZip string, excludeDirs []string) error {
	zipFile, err := os.Create(destZip)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if we should skip this directory
		for _, exclude := range excludeDirs {
			if info.IsDir() && info.Name() == exclude {
				return filepath.SkipDir
			}
		}

		// Skip the root dir itself
		if path == sourceDir {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Preserve directory structure
		relPath, _ := filepath.Rel(sourceDir, path)
		header.Name = filepath.ToSlash(relPath)

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	return nil
}
