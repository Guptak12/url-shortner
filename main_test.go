package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Each test gets a real SQLite file, separate from the application's database.
func newTestStore(t *testing.T) (*URLStore, string) {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "urls.db")
	store := openTestStore(t, filename)
	_, err := store.db.Exec(`CREATE TABLE urls (
		id TEXT PRIMARY KEY NOT NULL,
		original_url TEXT NOT NULL,
		short_url TEXT NOT NULL,
		creation_date TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		click_count INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	return store, filename
}

func openTestStore(t *testing.T, filename string) *URLStore {
	t.Helper()
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	if err := db.Ping(); err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	return &URLStore{db: db}
}

func TestValidateURLRejectsEmpty(t *testing.T) {
	err := validateURL("")
	if err == nil {
		t.Fatal("expected an error for an empty URL")
	}
}

func TestValidateURLRejectsInvalidScheme(t *testing.T) {
	err := validateURL("ftp://example.com")
	if err == nil {
		t.Fatal("expected an error for an invalid scheme")
	}
}

func TestValidateURLRejectsNoScheme(t *testing.T) {
	err := validateURL("example.com")
	if err == nil {
		t.Fatal("expected an error for a URL without a scheme")
	}
}

func TestValidateURLRejectsNoHost(t *testing.T) {
	err := validateURL("http://")
	if err == nil {
		t.Fatal("expected an error for a URL without a host")
	}
}

func TestValidateURLAcceptsValidHTTP(t *testing.T) {
	err := validateURL("http://example.com")
	if err != nil {
		t.Fatalf("expected no error for a valid HTTP URL, got: %v", err)
	}
}

func TestValidateURLAcceptsValidHTTPS(t *testing.T) {
	err := validateURL("https://example.com")
	if err != nil {
		t.Fatalf("expected no error for a valid HTTPS URL, got: %v", err)
	}
}

func TestValidateURLParsingError(t *testing.T) {
	err := validateURL("https://example.com/%zz")
	if err == nil {
		t.Fatal("expected an error for a URL with Parsing error")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	// Create a request with a method other than POST
	req, err := http.NewRequest(http.MethodGet, "/shorten", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Call the shortUrlHandler function
	store, _ := newTestStore(t)
	handler := shortUrlHandler(store)
	handler(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusMethodNotAllowed)
	}

	// Check the Allow header is set correctly.
	if allow := rr.Header().Get("Allow"); allow != http.MethodPost {
		t.Errorf("handler returned wrong Allow header: got %v want %v",
			allow, http.MethodPost)
	}
}

func TestShortUrlHandlerInvalidPayload(t *testing.T) {

	req := httptest.NewRequest(http.MethodPost, "/shorten", nil)

	rr := httptest.NewRecorder()
	store, _ := newTestStore(t)
	handler := shortUrlHandler(store)
	handler(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestShortUrlHandlerMalformedJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"http://example.com"`))

	rr := httptest.NewRecorder()
	store, _ := newTestStore(t)
	handler := shortUrlHandler(store)
	handler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d",
			rr.Code, http.StatusBadRequest)
	}
}

func TestShortUrlHandlerInvalidURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"invalid-url"}`))

	rr := httptest.NewRecorder()
	store, _ := newTestStore(t)
	handler := shortUrlHandler(store)
	handler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d",
			rr.Code, http.StatusBadRequest)
	}
}

func TestShortUrlHandlerValidURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"http://example.com"}`))

	rr := httptest.NewRecorder()
	store, _ := newTestStore(t)
	handler := shortUrlHandler(store)
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d",
			rr.Code, http.StatusOK)
	}

	var response struct {
		ShortURL string `json:"short_url"`
	}

	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}

	if response.ShortURL == "" {
		t.Fatal("expected a non-empty short_url")
	}
}

func TestCreateShortURLPersistsRow(t *testing.T) {
	store, _ := newTestStore(t)
	original := "https://example.com"
	created, err := store.createShortURL(original)
	if err != nil {
		t.Fatalf("create URL: %v", err)
	}
	if created.ID == "" || created.ShortURL != created.ID || created.OriginalURL != original {
		t.Fatalf("unexpected created URL: %+v", created)
	}
	if created.CreationDate.IsZero() || created.ClickCount != 0 {
		t.Fatalf("unexpected creation date or click count: %+v", created)
	}

	var originalURL, shortURL, createdAt string
	var clicks int
	err = store.db.QueryRow(
		"SELECT original_url, short_url, creation_date, click_count FROM urls WHERE id = ?",
		created.ID,
	).Scan(&originalURL, &shortURL, &createdAt, &clicks)
	if err != nil {
		t.Fatalf("read stored row: %v", err)
	}
	if originalURL != original || shortURL != created.ShortURL || clicks != 0 {
		t.Fatalf("unexpected stored row: original=%q short=%q clicks=%d", originalURL, shortURL, clicks)
	}
	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		t.Fatalf("parse stored timestamp: %v", err)
	}
	// RFC3339 storage has second precision.
	if !parsed.Equal(created.CreationDate.Truncate(time.Second)) {
		t.Fatalf("stored timestamp %v does not match creation time %v", parsed, created.CreationDate)
	}
}

func TestCreateShortURLReturnsExistingRow(t *testing.T) {
	store, _ := newTestStore(t)
	original := "https://example.com/existing"
	id := generateShortURL(original)
	createdAt := "2026-01-01T00:00:00Z"
	_, err := store.db.Exec(
		"INSERT INTO urls (id, original_url, short_url, creation_date, click_count) VALUES (?, ?, ?, ?, ?)",
		id, original, id, createdAt, 7,
	)
	if err != nil {
		t.Fatalf("seed existing row: %v", err)
	}

	got, err := store.createShortURL(original)
	if err != nil {
		t.Fatalf("get existing URL: %v", err)
	}
	if got.ID != id || got.ShortURL != id || got.OriginalURL != original || got.ClickCount != 7 {
		t.Fatalf("unexpected existing URL: %+v", got)
	}
	if got.CreationDate.Format(time.RFC3339) != createdAt {
		t.Fatalf("creation date changed: %v", got.CreationDate)
	}
	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM urls").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("got %d rows, want 1", count)
	}
}

func TestCreateShortURLRejectsInvalidURL(t *testing.T) {
	store, _ := newTestStore(t)
	if _, err := store.createShortURL("invalid-url"); err == nil {
		t.Fatal("expected invalid URL error")
	}
	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM urls").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("invalid URL created %d rows", count)
	}
}

func TestURLPersistsAfterReopeningDatabase(t *testing.T) {
	store, filename := newTestStore(t)
	created, err := store.createShortURL("https://example.com/persistent")
	if err != nil {
		t.Fatalf("create URL: %v", err)
	}
	if err := store.urlIncrementClicks(created.ID); err != nil {
		t.Fatalf("increment clicks: %v", err)
	}
	if err := store.db.Close(); err != nil {
		t.Fatalf("close original connection: %v", err)
	}

	reopened := openTestStore(t, filename)
	original, err := reopened.getOriginalURL(created.ID)
	if err != nil || original != created.OriginalURL {
		t.Fatalf("lookup after reopen: got %q, error %v", original, err)
	}
	loaded, err := reopened.createShortURL(created.OriginalURL)
	if err != nil {
		t.Fatalf("load existing URL after reopen: %v", err)
	}
	if loaded.ID != created.ID || loaded.ShortURL != created.ShortURL || loaded.ClickCount != 1 {
		t.Fatalf("unexpected persisted URL: %+v", loaded)
	}
	if !loaded.CreationDate.Equal(created.CreationDate.Truncate(time.Second)) {
		t.Fatalf("creation date did not persist: %v", loaded.CreationDate)
	}
}

func TestGetOriginalURLMissing(t *testing.T) {
	store, _ := newTestStore(t)
	got, err := store.getOriginalURL("missing")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("got error %v, want sql.ErrNoRows", err)
	}
	if got != "" {
		t.Fatalf("got URL %q for missing ID", got)
	}
}

func TestConcurrentURLAccess(t *testing.T) {
	store, _ := newTestStore(t)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			original := fmt.Sprintf("https://example.com/%d", i)
			short, err := store.createShortURL(original)
			if err != nil {
				t.Errorf("create URL %d: %v", i, err)
				return
			}
			got, err := store.getOriginalURL(short.ID)
			if err != nil {
				t.Errorf("lookup failed: %v", err)
				return
			}
			if got != original {
				t.Errorf("got %q, want %q", got, original)
			}
		}(i)
	}
	wg.Wait()
}

func TestClickIncrement(t *testing.T) {
	store, _ := newTestStore(t)
	// Omit creation_date and click_count to exercise the schema defaults.
	_, err := store.db.Exec(
		"INSERT INTO urls (id, original_url, short_url) VALUES (?, ?, ?)",
		"abc123", "https://example.com", "abc123",
	)
	if err != nil {
		t.Fatalf("seed URL: %v", err)
	}
	if err := store.urlIncrementClicks("abc123"); err != nil {
		t.Fatalf("increment clicks: %v", err)
	}
	var clicks int
	if err := store.db.QueryRow("SELECT click_count FROM urls WHERE id = ?", "abc123").Scan(&clicks); err != nil {
		t.Fatalf("read click count: %v", err)
	}
	if clicks != 1 {
		t.Fatalf("got click count %d, want 1", clicks)
	}
}

func TestClickIncrementMissingURL(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.urlIncrementClicks("missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("got error %v, want sql.ErrNoRows", err)
	}
}

func TestConcurrentClickIncrements(t *testing.T) {
	store, _ := newTestStore(t)
	created, err := store.createShortURL("https://example.com/clicks")
	if err != nil {
		t.Fatal(err)
	}
	const requests = 100
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := store.urlIncrementClicks(created.ID); err != nil {
				t.Errorf("increment clicks: %v", err)
			}
		}()
	}
	wg.Wait()

	var clicks int
	if err := store.db.QueryRow("SELECT click_count FROM urls WHERE id = ?", created.ID).Scan(&clicks); err != nil {
		t.Fatal(err)
	}
	if clicks != requests {
		t.Fatalf("got %d clicks, want %d", clicks, requests)
	}
}

func TestRedirectIncrementsClickCount(t *testing.T) {
	store, _ := newTestStore(t)
	created, err := store.createShortURL("https://example.com")
	if err != nil {
		t.Fatalf("create URL: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/redirect/"+created.ID, nil)
	rr := httptest.NewRecorder()
	redirectHandler(store)(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != created.OriginalURL {
		t.Fatalf("unexpected redirect location: %q", got)
	}
	var clicks int
	if err := store.db.QueryRow("SELECT click_count FROM urls WHERE id = ?", created.ID).Scan(&clicks); err != nil {
		t.Fatal(err)
	}
	if clicks != 1 {
		t.Fatalf("expected click count 1, got %d", clicks)
	}
}

func TestRedirectMissingURL(t *testing.T) {
	store, _ := newTestStore(t)
	req := httptest.NewRequest(http.MethodGet, "/redirect/missing", nil)
	rr := httptest.NewRecorder()
	redirectHandler(store)(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want 404", rr.Code)
	}
}

func TestShortURLHandlerDatabaseFailure(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.db.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"https://example.com"}`))
	rr := httptest.NewRecorder()
	shortUrlHandler(store)(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want 500", rr.Code)
	}
}
