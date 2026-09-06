---
title: Cookbook
description: The parts every app needs and no framework decides for you — database, session, upload, pagination, e-mail, scheduled task, Docker — with code that compiles.
---

"Learn" teaches the framework and "Reference" describes each symbol. This section answers a
third kind of question, the one that shows up on the second day: *how do I do the thing every
app does?* Open a database, keep someone logged in, receive a file, paginate a list, send an
e-mail, run a task every hour, put the whole thing in a container.

None of that is the framework's decision. Trilha has no ORM, no session store and no mailer —
what it has is a place for yours, and this is where the placing is written down.

| Recipe | What it answers |
|---|---|
| [Database](/cookbook/database) | pool, queries, transaction, migrations, sqlc |
| [Sessions](/cookbook/sessions) | login, signed cookie, current user, flash |
| [Uploads](/cookbook/uploads) | receive a file, validate it, store it, hand it back |
| [Pagination](/cookbook/pagination) | page and cursor, and the footer that goes with them |
| [E-mail](/cookbook/email) | SMTP in production, the log in dev, a body from a template |
| [Scheduled tasks](/cookbook/scheduled-tasks) | a ticker that starts with the app and stops with it |
| [Docker](/cookbook/docker) | a small image, the variables, the health probe |
| [Production checklist](/cookbook/production-checklist) | what to check before publishing, in order |
| [Migration](/cookbook/migration) | plain `net/http` to Trilha, and between minor versions |

## Where the code comes from

Every Go block on these pages is copied from a file in
[`examples/cookbook`](https://github.com/emersonjoe/trilha/tree/main/examples/cookbook), which
is part of the repository's module: `go vet ./...` compiles it on every run, and a site test
checks that each block still appears, character for character, in the file it came from. A
recipe that stops compiling breaks the build before it can mislead anyone.

That has a price worth knowing about: the package uses the standard library only, like the
rest of the repository. So there is no database driver, no password hash and no metrics
client in it. Where one is needed, the page says which line to add and why it is not here.

:::note
The recipes assume the conventions from [Pages and routes](/learn/pages-and-routes) and the
`app/setup.go` from [App](/reference/app). If a snippet mentions `Setup`, it belongs in that
file; if it mentions `Config`, it runs before the app exists.
:::
