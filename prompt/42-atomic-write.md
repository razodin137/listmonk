Your job is to go in and write each exported file atomically here to this Go app.

Write to a temporary file beside the destination and rename it into place, so a crash or a full disk never leaves half a post in the content directory.

Done is when make test runs clean and so do house-style lints

/home/mrjohn/Work/tries/2026-11-03-house-style/house-style
