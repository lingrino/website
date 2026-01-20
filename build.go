package main

import (
	"bufio"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"gopkg.in/yaml.v3"
)

const (
	contentDir   = "content"
	templatesDir = "templates"
	staticDir    = "static"
	outputDir    = "public"
	journalFile  = "journal/journal.txt"
)

func logError(err error) {
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func main() {
	builder, err := newBuilder()
	logError(err)

	err = builder.build()
	logError(err)
}

// Page represents a content page
type Page struct {
	Title       string
	Description string
	Date        string
	Content     template.HTML
	URL         string
	Slug        string
	Template    string
	Draft       bool
}

// SiteData holds global site data
type SiteData struct {
	JournalEntries []journal
	BlogPosts      []blogPost
}

// FeedJournalEntries returns the most recent 50 journal entries for feeds
func (s *SiteData) FeedJournalEntries() []journal {
	if len(s.JournalEntries) <= 50 {
		return s.JournalEntries
	}
	return s.JournalEntries[:50]
}

// FeedBlogPosts returns the most recent 50 blog posts for feeds
func (s *SiteData) FeedBlogPosts() []blogPost {
	if len(s.BlogPosts) <= 50 {
		return s.BlogPosts
	}
	return s.BlogPosts[:50]
}

// LatestJournalDateAtom returns the most recent journal entry date in Atom format
func (s *SiteData) LatestJournalDateAtom() string {
	if len(s.JournalEntries) == 0 {
		return time.Now().Format(time.RFC3339)
	}
	return s.JournalEntries[0].DateAtom
}

// LatestBlogDateAtom returns the most recent blog post date in Atom format
func (s *SiteData) LatestBlogDateAtom() string {
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

// TemplateData is passed to templates
type TemplateData struct {
	Page *Page
	Site *SiteData
}

type journal struct {
	Timestamp int64
	Date      string
	DateRSS   string // RFC 822 format for RSS
	DateAtom  string // RFC 3339 format for Atom
	URL       string
}

type blogPost struct {
	Title    string
	Slug     string        // filename without extension, for URL generation
	Date     string        // YYYY-MM-DD format
	DateRSS  string        // RFC 822 format for RSS
	DateAtom string        // RFC 3339 format for Atom
	Content  template.HTML // full HTML content
}

// Builder handles the site build process
type Builder struct {
	templates map[string]*template.Template
	site      *SiteData
}

func newBuilder() (*Builder, error) {
	je, err := loadJournal()
	if err != nil {
		return nil, err
	}

	b := &Builder{
		templates: make(map[string]*template.Template),
		site: &SiteData{
			JournalEntries: je,
			BlogPosts:      []blogPost{},
		},
	}

	err = b.loadTemplates()
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (b *Builder) loadTemplates() error {
	basePath := filepath.Join(templatesDir, "base.html")

	// Load page templates that extend base
	pageTemplates := []string{"home.html", "page.html", "blog-post.html", "blog-index.html", "journal.html"}
	for _, name := range pageTemplates {
		tmplPath := filepath.Join(templatesDir, name)
		tmpl, err := template.ParseFiles(basePath, tmplPath)
		if err != nil {
			return err
		}
		// Store without .html extension
		b.templates[strings.TrimSuffix(name, ".html")] = tmpl
	}

	return nil
}

func (b *Builder) build() error {
	// Create output directory
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		return err
	}

	// Copy static files
	err = b.copyStatic()
	if err != nil {
		return err
	}

	// First pass: collect all pages and blog posts
	pages, err := b.collectContent()
	if err != nil {
		return err
	}

	// Sort blog posts by date (newest first)
	slices.SortStableFunc(b.site.BlogPosts, func(a, b blogPost) int {
		return strings.Compare(b.Date, a.Date)
	})

	// Second pass: render all pages (now that BlogPosts is populated)
	err = b.renderPages(pages)
	if err != nil {
		return err
	}

	// Build feeds
	err = b.buildFeeds()
	if err != nil {
		return err
	}

	return nil
}

func (b *Builder) copyStatic() error {
	return filepath.WalkDir(staticDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath := strings.TrimPrefix(path, staticDir)
		if relPath == "" {
			return nil
		}
		relPath = strings.TrimPrefix(relPath, string(filepath.Separator))
		outputPath := filepath.Join(outputDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(outputPath, 0755)
		}

		return copyFile(path, outputPath)
	})
}

// pageInfo holds page data and metadata for two-pass processing
type pageInfo struct {
	page         *Page
	path         string
	outputPath   string
	templateName string
}

func (b *Builder) collectContent() ([]pageInfo, error) {
	var pages []pageInfo

	err := filepath.WalkDir(contentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		info, err := b.collectPage(path)
		if err != nil {
			return err
		}
		if info != nil {
			pages = append(pages, *info)
		}
		return nil
	})

	return pages, err
}

func (b *Builder) collectPage(path string) (*pageInfo, error) {
	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse frontmatter
	page, mdContent, err := parseFrontmatter(content)
	if err != nil {
		return nil, err
	}

	// Skip drafts
	if page.Draft {
		return nil, nil
	}

	// Convert markdown to HTML
	htmlContent := renderMarkdown(mdContent)
	page.Content = template.HTML(htmlContent)

	// Determine template
	templateName := b.determineTemplate(path, page)

	// Determine output path and URL
	outputPath := b.determineOutputPath(path)
	page.URL = b.determineURL(path)
	page.Slug = b.determineSlug(path)

	// For blog posts, derive title from filename if not set
	relPath := strings.TrimPrefix(path, contentDir+string(filepath.Separator))
	isBlogPost := strings.HasPrefix(relPath, "blog"+string(filepath.Separator)) && !strings.HasSuffix(relPath, "index.md")

	if isBlogPost && page.Title == "Untitled" {
		// Derive title from filename
		page.Title = strings.ReplaceAll(page.Slug, "-", " ")
	}

	// If this is a blog post (not index.md), add to blog posts list
	if isBlogPost {
		post := blogPost{
			Title:   page.Title,
			Slug:    page.Slug,
			Date:    page.Date,
			Content: page.Content,
		}

		// Format dates for RSS and Atom
		if page.Date != "" {
			location, err := time.LoadLocation("America/Los_Angeles")
			if err != nil {
				return nil, err
			}
			t, err := time.ParseInLocation(time.DateOnly, page.Date, location)
			if err != nil {
				return nil, err
			}
			post.DateRSS = t.Format(time.RFC1123Z)
			post.DateAtom = t.Format(time.RFC3339)
		}

		b.site.BlogPosts = append(b.site.BlogPosts, post)
	}

	return &pageInfo{
		page:         page,
		path:         path,
		outputPath:   outputPath,
		templateName: templateName,
	}, nil
}

func (b *Builder) renderPages(pages []pageInfo) error {
	for _, info := range pages {
		// Create output directory
		outputDirPath := filepath.Dir(info.outputPath)
		err := os.MkdirAll(outputDirPath, 0755)
		if err != nil {
			return err
		}

		// Render template
		tmpl, ok := b.templates[info.templateName]
		if !ok {
			slog.Warn("template not found, using page template", "template", info.templateName, "path", info.path)
			tmpl = b.templates["page"]
		}

		file, err := os.Create(info.outputPath)
		if err != nil {
			return err
		}

		data := TemplateData{
			Page: info.page,
			Site: b.site,
		}

		err = tmpl.Execute(file, data)
		file.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) determineTemplate(path string, page *Page) string {
	// Check frontmatter override
	if page.Template != "" {
		return page.Template
	}

	// Auto-detect based on path
	relPath := strings.TrimPrefix(path, contentDir+string(filepath.Separator))

	// Homepage
	if relPath == "index.md" {
		return "home"
	}

	// Journal
	if strings.HasPrefix(relPath, "journal"+string(filepath.Separator)) {
		return "journal"
	}

	// Blog index
	if relPath == "blog"+string(filepath.Separator)+"index.md" {
		return "blog-index"
	}

	// Blog post
	if strings.HasPrefix(relPath, "blog"+string(filepath.Separator)) {
		return "blog-post"
	}

	return "page"
}

func (b *Builder) determineOutputPath(path string) string {
	relPath := strings.TrimPrefix(path, contentDir+string(filepath.Separator))

	// index.md -> index.html in same directory
	if strings.HasSuffix(relPath, "index.md") {
		dir := strings.TrimSuffix(relPath, "index.md")
		return filepath.Join(outputDir, dir, "index.html")
	}

	// Regular .md -> slug/index.html for clean URLs
	slug := strings.TrimSuffix(filepath.Base(relPath), ".md")
	dir := filepath.Dir(relPath)
	return filepath.Join(outputDir, dir, slug, "index.html")
}

func (b *Builder) determineURL(path string) string {
	relPath := strings.TrimPrefix(path, contentDir+string(filepath.Separator))

	// index.md -> /dir/
	if strings.HasSuffix(relPath, "index.md") {
		dir := strings.TrimSuffix(relPath, "index.md")
		if dir == "" {
			return "/"
		}
		return "/" + strings.ReplaceAll(dir, string(filepath.Separator), "/")
	}

	// Regular .md -> /dir/slug/
	slug := strings.TrimSuffix(filepath.Base(relPath), ".md")
	dir := filepath.Dir(relPath)
	return "/" + strings.ReplaceAll(dir, string(filepath.Separator), "/") + "/" + slug + "/"
}

func (b *Builder) determineSlug(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".md")
}

func (b *Builder) buildFeeds() error {
	feedTemplates := []struct {
		src  string
		dest string
	}{
		{"feeds/journal.xml", "journal.xml"},
		{"feeds/journal.atom", "journal.atom"},
		{"feeds/blog.xml", "blog.xml"},
		{"feeds/blog.atom", "blog.atom"},
	}

	funcMap := texttemplate.FuncMap{
		"xml": escapeXML,
	}

	for _, ft := range feedTemplates {
		tmplPath := filepath.Join(templatesDir, ft.src)
		tmpl, err := texttemplate.New(filepath.Base(ft.src)).Funcs(funcMap).ParseFiles(tmplPath)
		if err != nil {
			return err
		}

		outputPath := filepath.Join(outputDir, ft.dest)
		file, err := os.Create(outputPath)
		if err != nil {
			return err
		}

		err = tmpl.Execute(file, b.site)
		file.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// Frontmatter represents YAML frontmatter
type Frontmatter struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Date        string `yaml:"date"`
	Template    string `yaml:"template"`
	Draft       bool   `yaml:"draft"`
}

func parseFrontmatter(content []byte) (*Page, []byte, error) {
	page := &Page{}

	str := string(content)
	if !strings.HasPrefix(str, "---\n") {
		return page, content, nil
	}

	// Find the closing ---
	rest := str[4:]
	endIndex := strings.Index(rest, "\n---\n")
	var frontmatterStr, remaining string

	if endIndex != -1 {
		frontmatterStr = rest[:endIndex]
		remaining = rest[endIndex+5:]
	} else if strings.HasSuffix(rest, "\n---") {
		frontmatterStr = rest[:len(rest)-4]
		remaining = ""
	} else {
		return page, content, nil
	}

	// Parse YAML
	var fm Frontmatter
	err := yaml.Unmarshal([]byte(frontmatterStr), &fm)
	if err != nil {
		return nil, nil, err
	}

	page.Title = fm.Title
	page.Description = fm.Description
	page.Date = fm.Date
	page.Template = fm.Template
	page.Draft = fm.Draft

	// If title is empty, derive from filename
	if page.Title == "" {
		page.Title = "Untitled"
	}

	return page, []byte(remaining), nil
}

func renderMarkdown(content []byte) []byte {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.SuperSubscript
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(content)

	htmlFlags := html.CommonFlags
	opts := html.RendererOptions{
		Flags:          htmlFlags,
		RenderNodeHook: renderLink,
	}
	renderer := html.NewRenderer(opts)

	return markdown.Render(doc, renderer)
}

// renderLink is a custom render hook that adds target="_blank" and rel="noopener"
// to external links (those starting with http:// or https://)
func renderLink(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
	link, ok := node.(*ast.Link)
	if !ok {
		return ast.GoToNext, false
	}

	if entering {
		dest := string(link.Destination)
		isExternal := strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://")

		if isExternal {
			fmt.Fprintf(w, `<a href="%s" target="_blank" rel="noopener">`, dest)
		} else {
			fmt.Fprintf(w, `<a href="%s">`, dest)
		}
	} else {
		io.WriteString(w, "</a>")
	}

	return ast.GoToNext, true
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Ensure parent directory exists
	err = os.MkdirAll(filepath.Dir(dst), 0755)
	if err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
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
		line := scanner.Text()
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			slog.Warn("skipping malformed journal entry", "line", line)
			continue
		}

		timestamp, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return nil, err
		}

		location, err := time.LoadLocation("America/Los_Angeles")
		if err != nil {
			return nil, err
		}
		t := time.Unix(timestamp, 0).In(location)
		date := t.Format(time.DateOnly)
		dateRSS := t.Format(time.RFC1123Z)
		dateAtom := t.Format(time.RFC3339)

		entries = append(entries, journal{
			Timestamp: timestamp,
			Date:      date,
			DateRSS:   dateRSS,
			DateAtom:  dateAtom,
			URL:       fields[1],
		})
	}

	err = scanner.Err()
	if err != nil {
		return nil, err
	}

	slices.SortStableFunc(entries, func(a, b journal) int {
		return int(b.Timestamp - a.Timestamp)
	})

	return entries, nil
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
