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
	BlogPosts      []blogPost
}

// FeedJournalEntries returns the most recent 50 journal entries for feeds
func (t *templater) FeedJournalEntries() []journal {
	if len(t.JournalEntries) <= 50 {
		return t.JournalEntries
	}
	return t.JournalEntries[:50]
}

// FeedBlogPosts returns the most recent 50 blog posts for feeds
func (t *templater) FeedBlogPosts() []blogPost {
	if len(t.BlogPosts) <= 50 {
		return t.BlogPosts
	}
	return t.BlogPosts[:50]
}

type journal struct {
	Timestamp int64
	Date      string
	DateRSS   string // RFC 822 format for RSS
	DateAtom  string // RFC 3339 format for Atom
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
	// First pass: collect blog posts and copy files
	err := filepath.WalkDir(inputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return os.MkdirAll(filepath.Join(outputDir, strings.TrimPrefix(path, inputDir)), 0755)
		}

		// Handle markdown files in blog directory
		relPath := strings.TrimPrefix(path, inputDir+string(filepath.Separator))
		if strings.HasPrefix(relPath, "blog"+string(filepath.Separator)) && strings.HasSuffix(path, ".md") {
			post, err := t.buildBlogPost(path)
			if err != nil {
				return err
			}
			t.BlogPosts = append(t.BlogPosts, *post)
			return nil
		}

		if strings.HasSuffix(path, ".tmpl") {
			// Skip templates that are not standalone pages
			base := filepath.Base(path)
			if base == "blog.html.tmpl" {
				return nil
			}
			// Skip feed templates for now - process after blog posts are collected
			if strings.HasSuffix(base, ".xml.tmpl") || strings.HasSuffix(base, ".atom.tmpl") {
				return nil
			}
			return t.copyTemplate(path)
		}

		return t.copyFile(path)
	})
	if err != nil {
		return err
	}

	// Sort blog posts by date (newest first)
	slices.SortStableFunc(t.BlogPosts, func(a, b blogPost) int {
		return strings.Compare(b.Date, a.Date)
	})

	// Second pass: process feed templates (now that BlogPosts is populated)
	return filepath.WalkDir(inputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasSuffix(base, ".xml.tmpl") || strings.HasSuffix(base, ".atom.tmpl") {
			return t.copyTemplate(path)
		}
		return nil
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

type blogPost struct {
	Title    string
	Slug     string        // filename without extension, for URL generation
	Date     string        // YYYY-MM-DD format
	DateRSS  string        // RFC 822 format for RSS
	DateAtom string        // RFC 3339 format for Atom
	Content  template.HTML // full HTML content
}

func (t *templater) buildBlogPost(path string) (*blogPost, error) {
	// Read markdown file
	mdContent, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse frontmatter and content
	content, date := parseFrontmatter(string(mdContent))

	// Convert markdown to HTML
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.SuperSubscript
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse([]byte(content))

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)
	htmlContent := markdown.Render(doc, renderer)

	// Extract title and slug from filename (remove .md extension)
	filename := filepath.Base(path)
	slug := strings.TrimSuffix(filename, ".md")
	title := strings.ReplaceAll(slug, "-", " ")

	// Format dates for RSS and Atom
	var dateRSS, dateAtom string
	if date != "" {
		location, err := time.LoadLocation("America/Los_Angeles")
		if err != nil {
			return nil, err
		}
		t, err := time.ParseInLocation(time.DateOnly, date, location)
		if err != nil {
			return nil, err
		}
		dateRSS = t.Format(time.RFC1123Z)
		dateAtom = t.Format(time.RFC3339)
	}

	post := &blogPost{
		Title:    title,
		Slug:     slug,
		Date:     date,
		DateRSS:  dateRSS,
		DateAtom: dateAtom,
		Content:  template.HTML(htmlContent),
	}

	// Load blog template
	tmplPath := filepath.Join(inputDir, "blog.html.tmpl")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		return nil, err
	}

	// Create output directory if needed
	relPath := strings.TrimPrefix(path, inputDir)
	outputPath := filepath.Join(outputDir, strings.TrimSuffix(relPath, ".md")+".html")
	outputDirPath := filepath.Dir(outputPath)
	err = os.MkdirAll(outputDirPath, 0755)
	if err != nil {
		return nil, err
	}

	// Generate HTML file
	file, err := os.Create(outputPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	err = tmpl.Execute(file, post)
	if err != nil {
		return nil, err
	}

	return post, nil
}

// parseFrontmatter extracts YAML frontmatter from markdown content
// Returns the content without frontmatter and the date if found
func parseFrontmatter(content string) (string, string) {
	if !strings.HasPrefix(content, "---\n") {
		return content, ""
	}

	// Find the closing ---
	endIndex := strings.Index(content[4:], "\n---\n")
	if endIndex == -1 {
		return content, ""
	}

	frontmatter := content[4 : 4+endIndex]
	remaining := content[4+endIndex+5:] // skip past \n---\n

	// Parse date from frontmatter
	var date string
	for _, line := range strings.Split(frontmatter, "\n") {
		if strings.HasPrefix(line, "date:") {
			date = strings.TrimSpace(strings.TrimPrefix(line, "date:"))
			break
		}
	}

	return remaining, date
}
