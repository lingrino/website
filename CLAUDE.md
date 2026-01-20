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
- Reads markdown content from `content/` directory
- Parses YAML frontmatter from markdown files
- Loads journal entries from `journal/journal.txt`
- Renders markdown to HTML using gomarkdown
- Applies HTML templates from `templates/` using Go's html/template
- Copies static files from `static/` unchanged
- Outputs everything to `public/` with clean URLs

**Directory structure:**
- `content/` - Markdown content with YAML frontmatter
  - `index.md` - Homepage
  - `journal/index.md` - Journal page
  - `blog/index.md` - Blog index
  - `blog/*.md` - Individual blog posts
- `templates/` - HTML templates
  - `base.html` - Common HTML boilerplate
  - `home.html`, `page.html`, `blog-post.html`, `blog-index.html`, `journal.html` - Page templates
  - `feeds/` - RSS/Atom feed templates
- `static/` - Static assets (CSS, fonts, robots.txt)
- `journal/journal.txt` - Journal entries as `<timestamp> <url>` lines
- `public/` - Generated output (gitignored)

**URL routing (clean URLs):**
- `content/index.md` → `public/index.html` → `/`
- `content/journal/index.md` → `public/journal/index.html` → `/journal/`
- `content/blog/index.md` → `public/blog/index.html` → `/blog/`
- `content/blog/my-post.md` → `public/blog/my-post/index.html` → `/blog/my-post/`

**Frontmatter schema:**
```yaml
---
title: "Page Title"
description: "SEO/feed description"
date: 2025-01-15           # Optional, used for sorting/feeds
template: page             # Optional, override auto-detection
draft: true                # Optional, skip during build
---
```

**Template data:**
- Templates receive `TemplateData` with `Page` and `Site` fields
- `Page`: Title, Description, Date, Content (rendered HTML), URL, Slug
- `Site`: JournalEntries, BlogPosts (sorted by date descending)
- Blog posts derive title from filename if not set in frontmatter

**Automation:**
- `journal.yml` workflow adds URLs to journal via workflow_dispatch, stripping tracking params
