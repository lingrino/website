package main

import (
	"cmp"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	texttemplate "text/template"
	"time"
)

const (
	contentDir  = "content"
	templatesDir = "templates"
	staticDir    = "static"
	outputDir    = "public"
	journalFile  = "journal/journal.txt"
	timezone     = "America/Los_Angeles"

	// feedEntryLimit caps RSS/Atom feeds to recent entries for performance
	feedEntryLimit = 50
)

// Template names
const (
	tmplHome      = "home"
	tmplPage      = "page"
	tmplBlogPost  = "blog-post"
	tmplBlogIndex = "blog-index"
	tmplJournal   = "journal"
)

// Content paths
const (
	pathBlogDir    = "blog"
	pathJournalDir = "journal"
)

// builder handles the site build process
type builder struct {
	templates     map[string]*template.Template
	feedTemplates map[string]*texttemplate.Template
	site          *siteData
	location      *time.Location
}

// newBuilder creates a new builder instance
func newBuilder() (*builder, error) {
	slog.Info("building site")

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("loading timezone: %w", err)
	}

	je, err := loadJournal(loc)
	if err != nil {
		return nil, fmt.Errorf("loading journal: %w", err)
	}

	b := &builder{
		templates:     make(map[string]*template.Template),
		feedTemplates: make(map[string]*texttemplate.Template),
		site: &siteData{
			JournalEntries: je,
			BlogPosts:      []blogPost{},
		},
		location: loc,
	}

	if err := b.loadTemplates(); err != nil {
		return nil, fmt.Errorf("loading templates: %w", err)
	}

	if err := b.loadFeedTemplates(); err != nil {
		return nil, fmt.Errorf("loading feed templates: %w", err)
	}

	return b, nil
}

// build executes the full build process
func (b *builder) build() error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	slog.Info("copying static files")
	if err := b.copyStatic(); err != nil {
		return fmt.Errorf("copying static files: %w", err)
	}

	slog.Info("processing content")
	pages, err := b.collectContent()
	if err != nil {
		return fmt.Errorf("collecting content: %w", err)
	}

	slices.SortStableFunc(b.site.BlogPosts, func(a, b blogPost) int {
		return cmp.Compare(b.Date, a.Date)
	})

	if err := b.renderPages(pages); err != nil {
		return fmt.Errorf("rendering pages: %w", err)
	}

	slog.Info("generating feeds")
	if err := b.buildFeeds(); err != nil {
		return fmt.Errorf("building feeds: %w", err)
	}

	slog.Info("build complete",
		"pages", len(pages),
		"blog_posts", len(b.site.BlogPosts),
		"journal_entries", len(b.site.JournalEntries))

	return nil
}

// copyStatic copies static files to output directory
func (b *builder) copyStatic() error {
	return filepath.WalkDir(staticDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel := strings.TrimPrefix(path, staticDir)
		if rel == "" {
			return nil
		}
		rel = strings.TrimPrefix(rel, string(filepath.Separator))
		outputPath := filepath.Join(outputDir, rel)

		if d.IsDir() {
			return os.MkdirAll(outputPath, 0755)
		}

		return copyFile(path, outputPath)
	})
}
