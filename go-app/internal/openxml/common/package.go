package common

import (
	"archive/zip"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/beevik/etree"
)

func UnzipToTemp(inputPath, prefix string) (string, error) {
	tempDir, err := os.MkdirTemp("", prefix)
	if err != nil {
		return "", err
	}

	reader, err := zip.OpenReader(inputPath)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", err
	}
	defer reader.Close()

	for _, file := range reader.File {
		targetPath := filepath.Join(tempDir, filepath.FromSlash(file.Name))
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				_ = os.RemoveAll(tempDir)
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			_ = os.RemoveAll(tempDir)
			return "", err
		}
		src, err := file.Open()
		if err != nil {
			_ = os.RemoveAll(tempDir)
			return "", err
		}
		dst, err := os.Create(targetPath)
		if err != nil {
			src.Close()
			_ = os.RemoveAll(tempDir)
			return "", err
		}
		if _, err := io.Copy(dst, src); err != nil {
			src.Close()
			dst.Close()
			_ = os.RemoveAll(tempDir)
			return "", err
		}
		src.Close()
		dst.Close()
	}
	return tempDir, nil
}

func ZipDir(sourceDir, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	defer writer.Close()

	return filepath.Walk(sourceDir, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(sourceDir, current)
		if err != nil {
			return err
		}
		zipName := filepath.ToSlash(relative)
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = zipName
		header.Method = zip.Deflate
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		src, err := os.Open(current)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(entry, src)
		return err
	})
}

func ResolveTarget(ownerPath, target string) string {
	ownerDir := path.Dir(filepath.ToSlash(ownerPath))
	return path.Clean(path.Join(ownerDir, target))
}

func RelativeTarget(ownerPath, targetPath string) string {
	ownerDir := path.Dir(filepath.ToSlash(ownerPath))
	relative, err := filepath.Rel(filepath.FromSlash(ownerDir), filepath.FromSlash(targetPath))
	if err != nil {
		return filepath.ToSlash(targetPath)
	}
	return filepath.ToSlash(relative)
}

func LocalName(tag string) string {
	if idx := strings.Index(tag, ":"); idx >= 0 {
		return tag[idx+1:]
	}
	return tag
}

func AttrName(attr etree.Attr) string {
	if attr.Space == "" {
		return attr.Key
	}
	return attr.Space + ":" + attr.Key
}

func GetAttr(element *etree.Element, preferred string, fallbacks ...string) (string, bool) {
	targets := append([]string{preferred}, fallbacks...)
	for _, attr := range element.Attr {
		current := AttrName(attr)
		for _, target := range targets {
			if current == target || attr.Key == target {
				return attr.Value, true
			}
		}
	}
	return "", false
}

func SetAttr(element *etree.Element, preferred, value string, fallbacks ...string) {
	targets := append([]string{preferred}, fallbacks...)
	for idx, attr := range element.Attr {
		current := AttrName(attr)
		for _, target := range targets {
			if current == target || attr.Key == target {
				element.Attr[idx].Value = value
				return
			}
		}
	}
	if parts := strings.SplitN(preferred, ":", 2); len(parts) == 2 {
		element.CreateAttr(parts[1], value).Space = parts[0]
		return
	}
	element.CreateAttr(preferred, value)
}
