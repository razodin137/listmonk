# Archives

A global public archive is maintained on the public web interface. It can be
enabled under Settings -> Settings -> General -> Enable public mailing list
archive.

To make a campaign available in the public archive (provided it has been
enabled in the settings as described above), enable the option
'Publish to public archive' under Campaigns -> Create new -> Archive.

A draft can also be published without sending it. Open the campaign's Archive
tab and click 'Publish to public archive'. The post is written to the live
archive, the dataset, the feed, and the exported files exactly as a sent
campaign is, while the campaign itself never goes through the mail queue.

When using template variables that depend on subscriber data (such as any
template variable referencing `.Subscriber`), such data must be supplied
as 'Campaign metadata', which is a JSON object that will be used in place
of `.Subscriber` when rendering the archive template and content.

When individual subscriber tracking is enabled, TrackLink requires that a UUID
of an existing user is provided as part of the campaign metadata. Any clicks on
a TrackLink from the archived campaign will be counted towards that subscriber.

As an example:

```json
{
  "UUID": "5a837423-a186-5623-9a87-82691cbe3631",
  "email": "example@example.com",
  "name": "Reader",
  "attribs": {}
}
```

![Archive campaign](images/archived-campaign-metadata.png)

## Exported files

Besides the live archive, every published post is written to disk as a
Markdown file with YAML front matter, ready for a Hugo-style site.

Files go to `content/posts` by default. Set `app.archive_export_dir` in
`config.toml` to change the directory. A relative path is resolved against the
directory containing the config file, and the directory is created if it does
not exist.

```toml
[app]
archive_export_dir = "content/posts"
```

## Exporting campaigns already sent

`--archive-export` writes the dataset and one Markdown file per campaign to the
given directory. Add `--archive-years` to limit the export to the last few
years; files are written newest first, and every campaign left out is logged
with the reason it was skipped.

```sh
./listmonk --archive-export content/posts --archive-years 3
```

`--archive-backfill` does the same while also publishing every finished
campaign that is not in the archive yet.

