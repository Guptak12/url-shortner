package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
	
)

type URL struct {
	ID           string    `json:"id"`
	OriginalURL  string    `json:"original_url"`
	ShortURL     string    `json:"short_url"`
	CreationDate time.Time `json:"creation_date"`
	ClickCount	int       `json:"click_count"`
}

type URLStore struct {
	urlDB map[string]URL
	mu   sync.RWMutex
}


var urlStore = URLStore{
	urlDB: make(map[string]URL),
	mu:    sync.RWMutex{},
}
// var urlDB = make(map[string]URL)
// var urlMu sync.RWMutex

func generateShortURL(originalURL string) string {
	hasher := sha256.New()
	hasher.Write([]byte(originalURL))
	// fmt.Println("Hashing original URL:", originalURL)
	hash := hasher.Sum(nil)
	// fmt.Println("Generated hash:", fmt.Sprintf("%x", hash))
	shortURL := fmt.Sprintf("%x", hash)[:8]
	return shortURL
}

func (s *URLStore) createShortURL(originalURL string) URL {
	shortUrl := generateShortURL(originalURL)
	s.mu.Lock()
	defer s.mu.Unlock()
	existing,exists := s.urlDB[shortUrl]
	if exists {
		return existing
	}
	url := URL{
		ID:           shortUrl,
		OriginalURL:  originalURL,
		ShortURL:     shortUrl,
		CreationDate: time.Now(),
		ClickCount:   0,
	}
	s.urlDB[shortUrl] = url
	return url
}

func getOriginalURL(shortURL string) (string, error) {
	urlStore.mu.RLock()
	url, exists := urlStore.urlDB[shortURL]
	urlStore.mu.RUnlock()
	if !exists {
		return "", fmt.Errorf("URL not found")
	}
	return url.OriginalURL, nil
}

func (s *URLStore) urlIncrementClicks(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	url, exists := s.urlDB[id]

	if !exists {
		return fmt.Errorf("URL Not Found")
	}	

	url.ClickCount++;
	s.urlDB[id]=url;
	return nil
}


func validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || (parsed.Hostname() == "") {
		return fmt.Errorf("invalid URL")
	}
	return nil

}

func (s *URLStore) saveUrls(filename string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	jsonBytes,err := json.Marshal(s.urlDB)
	if err !=nil{
		return fmt.Errorf("Error Parsing JSON")
	}

	// jsonStr := string(jsonBytes)
	file,err:= os.CreateTemp(filepath.Dir(filename), "tmp.json")
	if err != nil {
		return fmt.Errorf("Error creating temporary file")
	}
	defer file.Close()
	defer os.Remove(file.Name())
	er:= os.WriteFile(file.Name(), jsonBytes, 0644)
	if er!=nil{
		return fmt.Errorf("Error in Writing to the File")
	}
	errors := os.Rename(file.Name(), filename)
	if errors != nil {
		return fmt.Errorf("Error in Renaming the File")
	}
	
	return nil

}

func loadUrls(filename string) error {
	data,err := os.ReadFile(filename)

	if err !=nil{
		return fmt.Errorf("read Url File :%w", err)
	}

	jsonerr:=json.Unmarshal(data,&urlStore.urlDB)
	if jsonerr!=nil{
		return fmt.Errorf("Error in Unmarshaling Json")
	}
	return nil
}


func main() {
	// generatedShortURL := createShortURL("https://example.com")
	// fmt.Println("Generated Short URL:", generatedShortURL)

	http.HandleFunc("/shorten", shortUrlHandler("urls.json"))
	http.HandleFunc("/redirect/", redirectHandler("urls.json"))
	loaderr:=loadUrls("urls.json")
	if loaderr!=nil{
		fmt.Println("Error Loading file: %w",loaderr)
	}
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}



}
