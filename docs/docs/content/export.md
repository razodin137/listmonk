# Exporting posts

Every published post is also written to disk as a Markdown file with YAML front
matter, ready for a Hugo-style site. The export writes the same content that
the live archive serves, so the site, the feed and the files agree.

## The directory

Files go to `content/posts` by default. Set `app.archive_export_dir` in
`config.toml` to change that:

```toml
[app]
archive_export_dir = "content/posts"
```

A relative path is resolved against the directory containing the config file.
An absolute path is used as is. The directory and any missing parents are
created on startup if they do not exist.

Each export writes:

- `<slug>.md` for every post, named from the same slug used by the archive URL
  and the front matter.
- `index.md`, a table of every exported post with its slug, title, date and
  filename. It is rewritten on each run, so it always describes the files
  currently in the directory.
- `archive.json`, the archive dataset.
- A static HTML site beside them: `index.html`, one `<slug>.html` page per
  post, `archive.xml`, and `style.css`.

The export is idempotent: files whose contents have not changed are left
untouched. It also never overwrites a hand-edited post or static page. If one
differs from what the export would write, the edited file is kept and the new
content goes to a numeric variant such as `<name>-2.md`, then `-3`, and so on.

## The front matter

Each post starts with a YAML block, a blank line, and then the campaign body:

```markdown
---
title: September update
slug: september-update
date: "2026-09-01T09:00:00Z"
description: What changed this month.
tags:
    - newsletter
    - product
draft: false
---

<h1>September update</h1>
<p>What changed this month.</p>
```

The fields are:

- `title`: the campaign name, falling back to the subject for older campaigns.
- `slug`: the stable archive slug, shared with the URL and file name.
- `date`: the campaign's start time, send time, or creation time, as an RFC 3339
  timestamp in UTC. Omitted when none of those timestamps exist.
- `description`: the plain text of the first paragraph of the body.
- `tags`: the campaign's tags, or an empty list when it has none.
- `draft`: always `false`.

The body is the campaign's already-rendered HTML, written byte for byte. It is
not escaped and not parsed as Markdown again, so HTML tags, entities and
literal Markdown characters in the email survive unchanged.

## The two commands

Both commands run the export and exit without starting the web server.

`--archive-export` writes the dataset, the Markdown files, the index and the
static site to the given directory:

```sh
./listmonk --archive-export content/posts
```

`--archive-backfill` first publishes every finished campaign that is not in the
archive yet, then writes the same files. In this version both flags publish
missing finished campaigns before writing; `--archive-backfill` names that
intent explicitly.

```sh
./listmonk --archive-backfill content/posts
```

Add `--archive-years N` to either command to limit the export to campaigns from
the last N years. Entries are written newest first. A year count of zero, or no
flag, exports everything.

```sh
./listmonk --archive-export content/posts --archive-years 3
```

Each campaign left out is logged with the reason it was skipped, along with a
summary of how many files were written, left unchanged and kept as hand edits.
