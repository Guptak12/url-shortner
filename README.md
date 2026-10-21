# URL Shortener in Go

A URL shortener built as a hands-on Go learning project. It creates short IDs for HTTP and HTTPS URLs, redirects visitors to the original address, and tracks redirect counts in SQLite.

## How I built it

I manually wrote the entire codebase, starting with a tutorial to understand the basics and then leveling up the project one step at a time. I used guided explanations, documentation, and code review to work through unfamiliar concepts while implementing the changes myself.

The project grew through these stages:

1. Built the initial HTTP server, URL hashing, and redirects with an in-memory map.
2. Added URL parsing, input validation, and HTTP method checks.
3. Built unit and HTTP handler tests for valid requests and failure cases.
4. Added JSON file saving and loading so links could survive restarts.
5. Learned about goroutines, data races, and mutexes to coordinate shared map access.
6. Explored temporary-file replacement for safer JSON saves and temporary directories for isolated tests.
7. Separated handlers from storage logic and introduced a `URLStore`.
8. Added click counting and tests for concurrent access.
9. Replaced the map and JSON persistence with SQLite using `database/sql`.
10. Adapted the test suite to use a separate temporary SQLite database for each test.

The test suite grew alongside the implementation, covering validation, HTTP behavior, database persistence, click counting, and concurrency. Building each piece helped me understand not just how to make the service work, but how to check its behavior and investigate failures.

## Current features

- HTTP and HTTPS URL validation.
- Deterministic eight-character IDs derived from SHA-256.
- Reuse of an existing stored link when its ID is found, preserving its creation date and count.
- HTTP redirects with click-count updates.
- SQLite persistence across application restarts.
- Parameterized SQL queries.
- Handlers that receive a `URLStore` instead of using a global database.
- Tests using Go's `testing` and `net/http/httptest` packages.

The `short_url` response currently contains an ID, not a complete clickable URL. Use that ID with `/redirect/{id}`.

## Requirements

- Go compatible with the version declared in `go.mod` (currently Go 1.26.4).
- CGO enabled and a C compiler available for `github.com/mattn/go-sqlite3`.
- The SQLite command-line tool for the initial schema setup below.

On macOS, the Xcode Command Line Tools provide a C compiler.

## Run locally

Run these commands from the project directory:

```sh
go mod download
```

The application currently opens `./urls.db` but does not create the table automatically. Initialize the schema before the first run:

```sh
sqlite3 urls.db <<'SQL'
CREATE TABLE IF NOT EXISTS urls (
    id TEXT PRIMARY KEY NOT NULL,
    original_url TEXT NOT NULL,
    short_url TEXT NOT NULL,
    creation_date TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    click_count INTEGER NOT NULL DEFAULT 0
);
SQL
```

Start the server:

```sh
go run .
```

The server listens on `http://localhost:8080`. Use `go run .` so both `main.go` and `handler.go` are included.

SQLite handles persistence directly; the current application does not load the older JSON file or automatically import its contents.

## API

### Create a short link

```sh
curl -i -X POST http://localhost:8080/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com"}'
```

A successful response is `200 OK` with JSON shaped like:

```json
{"short_url":"<eight-character-id>"}
```

The endpoint returns `400` for malformed JSON or an invalid URL, `405` for methods other than POST, and `500` if link creation fails in storage.

### Follow a short link

Replace `<id>` with the returned ID:

```sh
curl -i 'http://localhost:8080/redirect/<id>'
```

A successful request increments the stored count and returns `302 Found` with the original URL in the `Location` header. An unknown ID returns `404`.

The count records successful handler increments, not unique people or confirmed visits to the destination.

### Inspect click counts

There is no statistics endpoint yet. Inspect stored counts locally with:

```sh
sqlite3 -header -column urls.db \
  'SELECT id, original_url, click_count FROM urls;'
```

## Tests

Run the suite:

```sh
go test -v ./...
```

Run with the race detector and bypass cached test results:

```sh
go test -race -count=1 ./...
```

The suite currently contains 24 tests covering:

- URL validation, including schemes, missing hosts, and parsing errors.
- Method rejection, empty bodies, malformed JSON, and successful shortening.
- Database inserts and retrieval of existing records.
- Persistence after closing and reopening the database.
- Missing IDs and database failure during shortening.
- Click increments, missing-link increments, and concurrent increments.
- Concurrent creation of different URLs and subsequent lookups.
- Redirect status, destination, click counting, and missing links.

Tests create their own schema and SQLite files inside `t.TempDir()`, so they do not write to the application's `urls.db`.

## Project structure

```text
main.go       URL model, SQLite store methods, validation, hashing, and startup
handler.go    Shortening and redirect HTTP handlers
main_test.go  Unit, handler, persistence, and concurrency tests
go.mod        Module and dependency declarations
go.sum        Dependency checksums
```

## Next improvements

This is a learning project with further improvements planned:

- Initialize the database schema during application startup.
- Normalize URL schemes and hostnames while preserving case-sensitive paths and query values.
- Detect short-ID collisions instead of assuming matching IDs identify the same URL.
- Handle simultaneous submissions of the same new URL without duplicate-insert failures.
- Distinguish database lookup failures from missing links in redirect responses.
- Return complete short URLs and expose link statistics through an endpoint.
- Add configuration, request logging, and graceful shutdown.

Passing tests cover the scenarios implemented in the suite; they do not mean every edge case is handled.
