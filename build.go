package main

import (
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type data struct {
	Time time.Time
}

func logError(err error) {
	if err != nil {
		slog.Error(err.Error())
	}
}

func copyTemplate(path string) error {
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join("public", strings.TrimSuffix(strings.TrimPrefix(path, "site"), ".tmpl")))
	if err != nil {
		return err
	}
	defer file.Close()

	return tmpl.Execute(file, data{Time: time.Now()})
}

func copyFile(path string) error {
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(filepath.Join("public", strings.TrimPrefix(path, "site")))
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)

	return err
}

func main() {
	err := os.MkdirAll("public", 0755)
	logError(err)

	err = filepath.WalkDir("site", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return os.MkdirAll(filepath.Join("public", strings.TrimPrefix(path, "site")), 0755)
		}

		if strings.HasSuffix(path, ".tmpl") {
			return copyTemplate(path)
		}

		return copyFile(path)
	})

	logError(err)
}
