# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
go run build.go      # Build the site (outputs to public/)
```

There are no tests or linting configured.

## Architecture

This is a static site generator for seanlingren.com written in Go.

**Build process (`build.go`):**
- Reads content from `site/` directory
- Loads journal entries from `journal/journal.txt`
- Processes `.tmpl` files with Go's `html/template` (strips `.tmpl` extension)
- Converts markdown files in `site/blog/` to HTML using `blog.html.tmpl` as wrapper
- Copies all other files unchanged
- Outputs everything to `public/`

**Content structure:**
- `site/` - Source files (HTML, templates, CSS, static assets)
- `site/blog/*.md` - Blog posts (filename becomes title, converted to HTML)
- `journal/journal.txt` - Journal entries as `<timestamp> <url>` lines, sorted by timestamp descending
- `public/` - Generated output (gitignored)

**Template data:**
- Templates receive a `templater` struct with `JournalEntries []journal`
- Each journal entry has `Timestamp`, `Date` (formatted for America/Los_Angeles), and `URL`
- Blog posts receive `Title` (from filename) and `Content` (rendered HTML)

**Automation:**
- `journal.yml` workflow adds URLs to journal via workflow_dispatch, stripping tracking params
