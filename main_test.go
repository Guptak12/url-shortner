package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)


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
	filename := filepath.Join(t.TempDir(), "urls.json")
handler := shortUrlHandler(filename)
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

	filename := filepath.Join(t.TempDir(), "urls.json")
	rr := httptest.NewRecorder()
	handler:= shortUrlHandler(filename)
	handler(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestShortUrlHandlerMalformedJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"http://example.com"`))
	

	rr := httptest.NewRecorder()
	filename := filepath.Join(t.TempDir(), "urls.json")
handler := shortUrlHandler(filename)
handler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d",
			rr.Code, http.StatusBadRequest)
	}
}

func TestShortUrlHandlerInvalidURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"invalid-url"}`))

	rr := httptest.NewRecorder()
	filename := filepath.Join(t.TempDir(), "urls.json")
handler := shortUrlHandler(filename)
handler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d",
			rr.Code, http.StatusBadRequest)
	}
}

func TestShortUrlHandlerValidURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"http://example.com"}`))

	rr := httptest.NewRecorder()
	filename := filepath.Join(t.TempDir(), "urls.json")
handler := shortUrlHandler(filename)
handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d",
			rr.Code, http.StatusOK)
	}
	// Add "encoding/json" to your imports.

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

func TestShortUrlLoading(t *testing.T){
	originalDB:=urlStore.urlDB

	t.Cleanup(func() {
		urlStore.urlDB = originalDB
	})
	urlStore.urlDB = make(map[string]URL)
	
	filename := filepath.Join(t.TempDir(), "urls.json")

	data := `{
		"abc123": {
			"id": "abc123",
			"original_url": "https://example.com",
			"short_url": "abc123",
			"creation_date": "2026-01-01T00:00:00Z",
			"click_count":0
		}
	}`

	if err := os.WriteFile(filename, []byte(data), 0600); err != nil {
		t.Fatalf("prepare test file: %v", err)
	}

	if err := loadUrls(filename); err != nil {
		t.Fatalf("loadUrls failed: %v", err)
	}

	loaded, exists := urlStore.urlDB["abc123"]
	if !exists {
		t.Fatal("expected abc123 to be loaded")
	}

	if loaded.OriginalURL != "https://example.com" {
		t.Fatalf("got URL %q, want %q",
			loaded.OriginalURL, "https://example.com")
	}

}

func TestShortUrlSaving(t *testing.T) {
	originalDB := urlStore.urlDB

	t.Cleanup(func() {
		urlStore.urlDB = originalDB
	})

	urlStore.urlDB = make(map[string]URL)

	urlStore.urlDB["abc123"] = URL{
		ID:          "abc123",
		OriginalURL: "https://example.com",
		ShortURL:    "abc123",
		ClickCount:  0,
	}

	filename := filepath.Join(t.TempDir(), "urls.json")

	if err := urlStore.saveUrls(filename); err != nil {
		t.Fatalf("saveUrls failed: %v", err)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}

	var loaded map[string]URL
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal saved data: %v", err)
	}

	if loaded["abc123"].OriginalURL != "https://example.com" {
		t.Fatalf("got URL %q, want %q",
			loaded["abc123"].OriginalURL, "https://example.com")
	}
}

func TestConcurrentURLAccess(t *testing.T) {
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			original := fmt.Sprintf("https://example.com/%d", i)
			short := urlStore.createShortURL(original)

			got, err := getOriginalURL(short.ID)
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
	originalDB := urlStore.urlDB

	t.Cleanup(func() {
		urlStore.urlDB = originalDB
	})

	urlStore.urlDB = make(map[string]URL)

	urlStore.urlDB["abc123"] = URL{
		ID:          "abc123",
		OriginalURL: "https://example.com",
		ShortURL:    "abc123",
		ClickCount:  0,
	}

	if err := urlStore.urlIncrementClicks("abc123"); err != nil {
		t.Fatalf("incrementClickCount failed: %v", err)
	}

	updated, exists := urlStore.urlDB["abc123"]
	if !exists {
		t.Fatal("expected abc123 to exist")
	}

	if updated.ClickCount != 1 {
		t.Fatalf("got click count %d, want %d", updated.ClickCount, 1)
	}
}

func TestRedirectIncrementsClickCount(t *testing.T) {
	originalDB := urlStore.urlDB

	t.Cleanup(func(){
		urlStore.urlDB=originalDB
	})
	urlStore.urlDB = make(map[string]URL)
	urlStore.urlDB["abc123"] = URL{
		ID:          "abc123",
		OriginalURL: "https://example.com",
		ShortURL:    "abc123",
		ClickCount:  0,
	}

	req := httptest.NewRequest(http.MethodGet, "/redirect/abc123", nil)
	rr := httptest.NewRecorder()

	filename := filepath.Join(t.TempDir(), "urls.json")
handler := redirectHandler(filename)
handler(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", rr.Code)
	}

	if got := rr.Header().Get("Location"); got != "https://example.com" {
		t.Fatalf("unexpected redirect location: %q", got)
	}

	if got := urlStore.urlDB["abc123"].ClickCount; got != 1 {
		t.Fatalf("expected click count 1, got %d", got)
	}

}