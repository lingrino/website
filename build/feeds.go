package main

import (
	"fmt"
	"path/filepath"
)

// feedMapping maps feed template paths to output file names
var feedMapping = []struct {
	template string
	output   string
}{
	{"feeds/journal.xml", "journal.xml"},
	{"feeds/journal.atom", "journal.atom"},
	{"feeds/blog.xml", "blog.xml"},
	{"feeds/blog.atom", "blog.atom"},
}

// buildFeeds generates RSS and Atom feeds
func (b *builder) buildFeeds() error {
	for _, fm := range feedMapping {
		tmpl, ok := b.feedTemplates[fm.template]
		if !ok {
			return fmt.Errorf("feed template %s not found", fm.template)
		}

		outputPath := filepath.Join(outputDir, fm.output)
		if err := writeTemplate(outputPath, tmpl, b.site); err != nil {
			return fmt.Errorf("rendering feed %s: %w", fm.output, err)
		}
	}

	return nil
}
