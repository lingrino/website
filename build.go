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

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
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
	Timestamp int64
	Date      string
	URL       string
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

		// Handle markdown files in blog directory
		relPath := strings.TrimPrefix(path, inputDir+string(filepath.Separator))
		if strings.HasPrefix(relPath, "blog"+string(filepath.Separator)) && strings.HasSuffix(path, ".md") {
			return t.buildBlogPost(path)
		}

		if strings.HasSuffix(path, ".tmpl") {
			// Skip blog.html.tmpl as it's only used for blog posts, not as a standalone page
			if filepath.Base(path) == "blog.html.tmpl" {
				return nil
			}
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

		location, err := time.LoadLocation("America/Los_Angeles")
		if err != nil {
			return nil, err
		}
		date := time.Unix(timestamp, 0).In(location).Format(time.DateOnly)

		entries = append(entries, journal{Timestamp: timestamp, Date: date, URL: fields[1]})
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

type blogPost struct {
	Title   string
	Content template.HTML
}

func (t *templater) buildBlogPost(path string) error {
	// Read markdown file
	mdContent, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Convert markdown to HTML
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.SuperSubscript
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(mdContent)

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)
	htmlContent := markdown.Render(doc, renderer)

	// Extract title from filename (remove .md extension)
	filename := filepath.Base(path)
	title := strings.TrimSuffix(filename, ".md")

	// Load blog template
	tmplPath := filepath.Join(inputDir, "blog.html.tmpl")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		return err
	}

	// Create output directory if needed
	relPath := strings.TrimPrefix(path, inputDir)
	outputPath := filepath.Join(outputDir, strings.TrimSuffix(relPath, ".md")+".html")
	outputDirPath := filepath.Dir(outputPath)
	err = os.MkdirAll(outputDirPath, 0755)
	if err != nil {
		return err
	}

	// Generate HTML file
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	post := blogPost{
		Title:   title,
		Content: template.HTML(htmlContent),
	}

	return tmpl.Execute(file, post)
}
