package main

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	texttemplate "text/template"

	"gopkg.in/yaml.v3"
)

// loadTemplates loads all HTML templates
func (b *builder) loadTemplates() error {
	basePath := filepath.Join(templatesDir, "base.html")

	pageTemplates := []string{
		tmplHome + ".html",
		tmplPage + ".html",
		tmplBlogPost + ".html",
		tmplBlogIndex + ".html",
		tmplJournal + ".html",
	}

	for _, name := range pageTemplates {
		tmplPath := filepath.Join(templatesDir, name)
		tmpl, err := template.ParseFiles(basePath, tmplPath)
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", name, err)
		}
		b.templates[strings.TrimSuffix(name, ".html")] = tmpl
	}

	return nil
}

// loadFeedTemplates loads all feed templates
func (b *builder) loadFeedTemplates() error {
	funcMap := texttemplate.FuncMap{
		"xml": escapeXML,
	}

	feedTemplates := []string{
		"feeds/journal.xml",
		"feeds/journal.atom",
		"feeds/blog.xml",
		"feeds/blog.atom",
	}

	for _, name := range feedTemplates {
		tmplPath := filepath.Join(templatesDir, name)
		tmpl, err := texttemplate.New(filepath.Base(name)).Funcs(funcMap).ParseFiles(tmplPath)
		if err != nil {
			return fmt.Errorf("parsing feed template %s: %w", name, err)
		}
		b.feedTemplates[name] = tmpl
	}

	return nil
}

// renderPages renders all collected pages
func (b *builder) renderPages(pages []pageInfo) error {
	for _, info := range pages {
		if err := os.MkdirAll(filepath.Dir(info.outputPath), 0755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", info.outputPath, err)
		}

		tmpl, ok := b.templates[info.templateName]
		if !ok {
			if info.page.Template != "" {
				return fmt.Errorf("template %q not found for %s", info.templateName, info.path)
			}
			tmpl = b.templates[tmplPage]
		}

		data := templateData{
			Page: info.page,
			Site: b.site,
		}

		if err := writeTemplate(info.outputPath, tmpl, data); err != nil {
			return fmt.Errorf("rendering %s: %w", info.path, err)
		}

		// Write markdown version of the page
		if err := b.writeMarkdownPage(info); err != nil {
			return fmt.Errorf("writing markdown for %s: %w", info.path, err)
		}
	}
	return nil
}

// writeMarkdownPage writes the markdown version of a page
func (b *builder) writeMarkdownPage(info pageInfo) error {
	// Determine markdown output path (same as HTML but with .md extension)
	mdOutputPath := strings.TrimSuffix(info.outputPath, ".html") + ".md"

	var mdContent []byte

	switch info.pathType {
	case pathJournal:
		mdContent = b.generateJournalMarkdown(info.page)
	case pathBlogIndex:
		mdContent = b.generateBlogIndexMarkdown(info.page)
	default:
		// For regular pages, use the original markdown source
		mdContent = info.page.MarkdownSource
	}

	// Ensure file ends with a newline
	if len(mdContent) > 0 && mdContent[len(mdContent)-1] != '\n' {
		mdContent = append(mdContent, '\n')
	}

	if err := os.MkdirAll(filepath.Dir(mdOutputPath), 0755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", mdOutputPath, err)
	}

	return os.WriteFile(mdOutputPath, mdContent, 0644)
}

// yamlScalar formats a string as a properly escaped YAML scalar value
func yamlScalar(s string) string {
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Sprintf("%q", s)
	}
	return strings.TrimSpace(string(data))
}

// writeFrontmatter writes YAML frontmatter with properly escaped values
func writeFrontmatter(sb *strings.Builder, pg *page) {
	sb.WriteString("---\n")
	if pg.Title != "" {
		sb.WriteString(fmt.Sprintf("title: %s\n", yamlScalar(pg.Title)))
	}
	if pg.Description != "" {
		sb.WriteString(fmt.Sprintf("description: %s\n", yamlScalar(pg.Description)))
	}
	if pg.Date != "" {
		sb.WriteString(fmt.Sprintf("date: %s\n", yamlScalar(pg.Date)))
	}
	sb.WriteString("---\n\n")
}

// generateJournalMarkdown generates markdown content for the journal page with entries
func (b *builder) generateJournalMarkdown(pg *page) []byte {
	var sb strings.Builder

	writeFrontmatter(&sb, pg)

	// Write heading
	sb.WriteString("# journal\n\n")

	// Write journal entries as a list
	for _, entry := range b.site.JournalEntries {
		sb.WriteString(fmt.Sprintf("- %s [%s](%s)\n", entry.Date, entry.URL, entry.URL))
	}

	return []byte(sb.String())
}

// generateBlogIndexMarkdown generates markdown content for the blog index with posts
func (b *builder) generateBlogIndexMarkdown(pg *page) []byte {
	var sb strings.Builder

	writeFrontmatter(&sb, pg)

	// Write heading
	sb.WriteString("# blog\n\n")

	// Write blog posts as a list
	for _, post := range b.site.BlogPosts {
		sb.WriteString(fmt.Sprintf("- %s [%s](/blog/%s)\n", post.Date, post.Title, post.Slug))
	}

	return []byte(sb.String())
}

// writeTemplate creates a file and executes a template to it
func writeTemplate[T interface{ Execute(w io.Writer, data any) error }](path string, tmpl T, data any) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	return tmpl.Execute(f, data)
}
