package main

import (
	"html/template"
	"time"
)

// page represents a content page
type page struct {
	Title          string
	Description    string
	Date           string
	Content        template.HTML
	MarkdownSource []byte // Raw markdown content with frontmatter
	URL            string
	Slug           string
	Template       string
	Draft          bool
}

// MarkdownURL returns the URL for the markdown version of this page
func (p *page) MarkdownURL() string {
	if p.URL == "/" {
		return "/index.md"
	}
	return p.URL + ".md"
}

// siteData holds global site data
type siteData struct {
	JournalEntries []journal
	BlogPosts      []blogPost
}

// FeedJournalEntries returns the most recent entries for feeds
func (s *siteData) FeedJournalEntries() []journal {
	if len(s.JournalEntries) <= feedEntryLimit {
		return s.JournalEntries
	}
	return s.JournalEntries[:feedEntryLimit]
}

// FeedBlogPosts returns the most recent blog posts for feeds
func (s *siteData) FeedBlogPosts() []blogPost {
	if len(s.BlogPosts) <= feedEntryLimit {
		return s.BlogPosts
	}
	return s.BlogPosts[:feedEntryLimit]
}

// LatestJournalDateAtom returns the most recent journal entry date in Atom format
func (s *siteData) LatestJournalDateAtom() string {
	if len(s.JournalEntries) == 0 {
		return time.Now().Format(time.RFC3339)
	}
	return s.JournalEntries[0].DateAtom
}

// LatestBlogDateAtom returns the most recent blog post date in Atom format
func (s *siteData) LatestBlogDateAtom() string {
	if len(s.BlogPosts) == 0 {
		return time.Now().Format(time.RFC3339)
	}
	for _, p := range s.BlogPosts {
		if p.DateAtom != "" {
			return p.DateAtom
		}
	}
	return time.Now().Format(time.RFC3339)
}

// templateData is passed to templates
type templateData struct {
	Page *page
	Site *siteData
}

// journal represents a journal entry
type journal struct {
	ID        string
	Timestamp int64
	Date      string
	DateRSS   string
	DateAtom  string
	URL       string
}

// blogPost represents a blog post
type blogPost struct {
	Title    string
	Slug     string
	Date     string
	DateRSS  string
	DateAtom string
	Content  template.HTML
}

// frontmatter represents YAML frontmatter
type frontmatter struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Date        string `yaml:"date"`
	Template    string `yaml:"template"`
	Draft       bool   `yaml:"draft"`
}

// pageInfo holds page data and metadata for two-pass processing
type pageInfo struct {
	page         *page
	path         string
	outputPath   string
	templateName string
	pathType     pathType
}

// pathType represents the type of content path
type pathType int

const (
	pathHome pathType = iota
	pathJournal
	pathBlogIndex
	pathBlogPost
	pathPage
)
