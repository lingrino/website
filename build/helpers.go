package main

import (
	"bufio"
	"cmp"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// relPath returns the path relative to contentDir
func relPath(path string) string {
	return strings.TrimPrefix(path, contentDir+string(filepath.Separator))
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", dst, err)
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying to %s: %w", dst, err)
	}
	return nil
}

// escapeXML escapes special XML characters in a string
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// parseDate parses a YYYY-MM-DD date string in the given timezone
func parseDate(s string, loc *time.Location) (time.Time, error) {
	t, err := time.ParseInLocation(time.DateOnly, s, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing date %q: %w", s, err)
	}
	return t, nil
}

// formatDateRSS formats time for RSS feeds (RFC1123Z)
func formatDateRSS(t time.Time) string {
	return t.Format(time.RFC1123Z)
}

// formatDateAtom formats time for Atom feeds (RFC3339)
func formatDateAtom(t time.Time) string {
	return t.Format(time.RFC3339)
}

// loadJournal reads and parses the journal file
func loadJournal(loc *time.Location) ([]journal, error) {
	file, err := os.Open(journalFile)
	if err != nil {
		return nil, fmt.Errorf("opening journal: %w", err)
	}
	defer file.Close()

	var entries []journal

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("malformed journal entry, expected '<timestamp> <url>', got: %q", line)
		}

		timestamp, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing timestamp %q: %w", fields[0], err)
		}

		t := time.Unix(timestamp, 0).In(loc)

		entries = append(entries, journal{
			Timestamp: timestamp,
			Date:      t.Format(time.DateOnly),
			DateRSS:   formatDateRSS(t),
			DateAtom:  formatDateAtom(t),
			URL:       fields[1],
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading journal: %w", err)
	}

	slices.SortStableFunc(entries, func(a, b journal) int {
		return cmp.Compare(b.Timestamp, a.Timestamp)
	})

	return entries, nil
}
