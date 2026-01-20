package main

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	texttemplate "text/template"
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
	}
	return nil
}

// writeTemplate creates a file and executes a template to it
func writeTemplate[T interface{ Execute(w io.Writer, data any) error }](path string, tmpl T, data any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, data)
}
