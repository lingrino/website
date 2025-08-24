package main

import (
	"bufio"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	inputDir    = "site"
	outputDir   = "public"
	journalFile = "journal/journal.txt"
)

func logError(err error) {
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func main() {
	templater, err := newTemplater()
	logError(err)

	err = os.MkdirAll(outputDir, 0755)
	logError(err)

	err = templater.build()
	logError(err)
}

type templater struct {
	JournalEntries []journal
}

type journal struct {
	Date string
	URL  string
}

func newTemplater() (*templater, error) {
	je, err := loadJournal()
	if err != nil {
		return nil, err
	}

	return &templater{JournalEntries: je}, nil
}

func (t *templater) build() error {
	return filepath.WalkDir(inputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return os.MkdirAll(filepath.Join(outputDir, strings.TrimPrefix(path, inputDir)), 0755)
		}

		if strings.HasSuffix(path, ".tmpl") {
			return t.copyTemplate(path)
		}

		return t.copyFile(path)
	})
}

func (t *templater) copyTemplate(path string) error {
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(outputDir, strings.TrimSuffix(strings.TrimPrefix(path, inputDir), ".tmpl")))
	if err != nil {
		return err
	}
	defer file.Close()

	return tmpl.Execute(file, t)
}

func (t *templater) copyFile(path string) error {
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(filepath.Join(outputDir, strings.TrimPrefix(path, inputDir)))
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)

	return err
}

func loadJournal() ([]journal, error) {
	file, err := os.Open(journalFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	entries := []journal{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		timestamp, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return nil, err
		}

		date := time.Unix(timestamp, 0).Format(time.DateOnly)

		entries = append(entries, journal{Date: date, URL: fields[1]})
	}

	err = scanner.Err()
	if err != nil {
		return nil, err
	}

	slices.SortStableFunc(entries, func(a, b journal) int {
		return strings.Compare(b.Date, a.Date)
	})

	return entries, nil
}
