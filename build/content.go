package main

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"gopkg.in/yaml.v3"
)

// collectContent walks the content directory and collects all pages
func (b *builder) collectContent() ([]pageInfo, error) {
	var pages []pageInfo

	err := filepath.WalkDir(contentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		info, err := b.collectPage(path)
		if err != nil {
			return fmt.Errorf("collecting %s: %w", path, err)
		}
		if info != nil {
			pages = append(pages, *info)
		}
		return nil
	})

	return pages, err
}

// collectPage processes a single content file
func (b *builder) collectPage(path string) (*pageInfo, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	pg, mdContent, err := parseFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	if pg.Draft {
		return nil, nil
	}

	pg.Content = template.HTML(renderMarkdown(mdContent))
	pg.URL = b.determineURL(path)
	pg.Slug = b.determineSlug(path)

	templateName := b.determineTemplate(path, pg)
	outputPath := b.determineOutputPath(path)

	pathClass := classifyPath(path)

	// For blog posts, derive title from filename if not set
	if pathClass == pathBlogPost && pg.Title == "" {
		pg.Title = strings.ReplaceAll(pg.Slug, "-", " ")
	}

	// Add blog posts to site data
	if pathClass == pathBlogPost {
		post := blogPost{
			Title:   pg.Title,
			Slug:    pg.Slug,
			Date:    pg.Date,
			Content: pg.Content,
		}

		if pg.Date != "" {
			t, err := parseDate(pg.Date, b.location)
			if err != nil {
				return nil, err
			}
			post.DateRSS = formatDateRSS(t)
			post.DateAtom = formatDateAtom(t)
		}

		b.site.BlogPosts = append(b.site.BlogPosts, post)
	}

	return &pageInfo{
		page:         pg,
		path:         path,
		outputPath:   outputPath,
		templateName: templateName,
	}, nil
}

// classifyPath determines the type of content based on path
func classifyPath(path string) pathType {
	rel := relPath(path)

	if rel == "index.md" {
		return pathHome
	}

	if strings.HasPrefix(rel, pathJournalDir+string(filepath.Separator)) {
		return pathJournal
	}

	if rel == pathBlogDir+string(filepath.Separator)+"index.md" {
		return pathBlogIndex
	}

	if strings.HasPrefix(rel, pathBlogDir+string(filepath.Separator)) {
		return pathBlogPost
	}

	return pathPage
}

// determineTemplate determines which template to use for a page
func (b *builder) determineTemplate(path string, pg *page) string {
	if pg.Template != "" {
		return pg.Template
	}

	switch classifyPath(path) {
	case pathHome:
		return tmplHome
	case pathJournal:
		return tmplJournal
	case pathBlogIndex:
		return tmplBlogIndex
	case pathBlogPost:
		return tmplBlogPost
	default:
		return tmplPage
	}
}

// isRootIndex returns true if the relative path is the root index.md
func isRootIndex(rel string) bool {
	return rel == "index.md"
}

// isDirIndex returns true if the path is a directory's index.md and extracts the directory name
func isDirIndex(rel string) (dir string, ok bool) {
	suffix := string(filepath.Separator) + "index.md"
	if strings.HasSuffix(rel, suffix) {
		return strings.TrimSuffix(rel, suffix), true
	}
	return "", false
}

// determineOutputPath determines the output file path
func (b *builder) determineOutputPath(path string) string {
	rel := relPath(path)

	if isRootIndex(rel) {
		return filepath.Join(outputDir, "index.html")
	}

	if dir, ok := isDirIndex(rel); ok {
		return filepath.Join(outputDir, dir+".html")
	}

	return filepath.Join(outputDir, strings.TrimSuffix(rel, ".md")+".html")
}

// determineURL determines the URL for a page
func (b *builder) determineURL(path string) string {
	rel := relPath(path)

	if isRootIndex(rel) {
		return "/"
	}

	if dir, ok := isDirIndex(rel); ok {
		return "/" + strings.ReplaceAll(dir, string(filepath.Separator), "/")
	}

	slug := strings.TrimSuffix(filepath.Base(rel), ".md")
	dir := filepath.Dir(rel)
	if dir == "." {
		return "/" + slug
	}
	return "/" + strings.ReplaceAll(dir, string(filepath.Separator), "/") + "/" + slug
}

// determineSlug extracts the slug from a path
func (b *builder) determineSlug(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".md")
}

// parseFrontmatter extracts frontmatter from content
func parseFrontmatter(content []byte) (*page, []byte, error) {
	pg := &page{}

	str := string(content)
	if !strings.HasPrefix(str, "---\n") {
		return pg, content, nil
	}

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
		return pg, content, nil
	}

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(frontmatterStr), &fm); err != nil {
		return nil, nil, fmt.Errorf("invalid YAML: %w", err)
	}

	if fm.Date != "" {
		if _, err := time.Parse(time.DateOnly, fm.Date); err != nil {
			return nil, nil, fmt.Errorf("invalid date format %q, expected YYYY-MM-DD: %w", fm.Date, err)
		}
	}

	pg.Title = fm.Title
	pg.Description = fm.Description
	pg.Date = fm.Date
	pg.Template = fm.Template
	pg.Draft = fm.Draft

	return pg, []byte(remaining), nil
}

// renderMarkdown converts markdown content to HTML
func renderMarkdown(content []byte) []byte {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.SuperSubscript
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(content)

	opts := html.RendererOptions{
		Flags:          html.CommonFlags,
		RenderNodeHook: renderLink,
	}
	renderer := html.NewRenderer(opts)

	return markdown.Render(doc, renderer)
}

// isSafeURL checks if a URL scheme is safe (not javascript:, data:, etc.)
func isSafeURL(dest string) bool {
	lower := strings.ToLower(dest)
	// Allow http, https, mailto, tel, and relative URLs
	if strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "tel:") {
		return true
	}
	// Block URLs with explicit schemes (javascript:, data:, vbscript:, etc.)
	if strings.Contains(lower, ":") && !strings.HasPrefix(lower, "/") {
		return false
	}
	// Allow relative URLs
	return true
}

// renderLink adds target="_blank" and rel="noopener" to external links
func renderLink(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
	link, ok := node.(*ast.Link)
	if !ok {
		return ast.GoToNext, false
	}

	if entering {
		dest := string(link.Destination)

		// Block potentially dangerous URI schemes
		if !isSafeURL(dest) {
			fmt.Fprint(w, `<a href="#">`)
			return ast.GoToNext, true
		}

		escapedDest := template.HTMLEscapeString(dest)
		isExternal := strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://")

		if isExternal {
			fmt.Fprintf(w, `<a href="%s" target="_blank" rel="noopener">`, escapedDest)
		} else {
			fmt.Fprintf(w, `<a href="%s">`, escapedDest)
		}
	} else {
		io.WriteString(w, "</a>")
	}

	return ast.GoToNext, true
}
