package main

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
	_ "github.com/mattn/go-sqlite3"
)

type URL struct {
	ID           string    `json:"id"`
	OriginalURL  string    `json:"original_url"`
	ShortURL     string    `json:"short_url"`
	CreationDate time.Time `json:"creation_date"`
	ClickCount   int       `json:"click_count"`
}

type URLStore struct {
	db *sql.DB
}

// var urlStore = URLStore{
// 	urlDB: make(map[string]URL),
// 	mu:    sync.RWMutex{},
// }
// var urlDB = make(map[string]URL)
// var urlMu sync.RWMutex

//////////////////////////////////////////////////////////
//////////// Internal functions for URLStore /////////////
//////////////////////////////////////////////////////////

func generateShortURL(originalURL string) string {
	// originalURL = strings.ToLower(originalURL)
	hasher := sha256.New()
	hasher.Write([]byte(originalURL))
	// fmt.Println("Hashing original URL:", originalURL)
	hash := hasher.Sum(nil)
	// fmt.Println("Generated hash:", fmt.Sprintf("%x", hash))
	shortURL := fmt.Sprintf("%x", hash)[:8]
	return shortURL
}

func validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || (parsed.Hostname() == "") {
		return fmt.Errorf("invalid URL")
	}
	return nil

}

func (s *URLStore) createShortURL(originalURL string) (URL,error) {
	if err := validateURL(originalURL); err != nil {
		return URL{}, err
	}

	shortUrl := generateShortURL(originalURL)

	url := URL{
		ID:           shortUrl,
		OriginalURL:  originalURL,
		ShortURL:     shortUrl,
		CreationDate: time.Now(),
		ClickCount:   0,
	}
	var createdAt string
	er:=s.db.QueryRow("SELECT id, original_url, short_url, creation_date, click_count FROM urls WHERE id = ?", shortUrl).Scan(&url.ID, &url.OriginalURL, &url.ShortURL, &createdAt, &url.ClickCount)

	if er == nil {
		parsedTime, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return URL{}, fmt.Errorf("Error parsing creation date: %w", err)
		}
		url.CreationDate = parsedTime
		return url, nil
	}

	if errors.Is(er, sql.ErrNoRows){
		_, err := s.db.Exec("INSERT INTO urls (id, original_url, short_url, creation_date, click_count) VALUES (?, ?, ?, ?, ?)",
			url.ID, url.OriginalURL, url.ShortURL, url.CreationDate.UTC().Format(time.RFC3339), url.ClickCount)

		if err != nil {
			return URL{}, fmt.Errorf("Error inserting URL into database: %w", err)
		}
		return url,nil
	}
	if er != nil {
		return URL{}, er
	}
	return url,nil
}

func (s *URLStore) getOriginalURL(id string) (string, error) {

	var originalURL string

	err := s.db.QueryRow(
		"SELECT original_url FROM urls WHERE id = ?",
		id,
	).Scan(&originalURL)

	return originalURL, err
}

func (s *URLStore) urlIncrementClicks(id string) error {

	result,er := s.db.Exec("UPDATE urls SET click_count = click_count + 1 WHERE id = ?", id)
	if er != nil {
		return fmt.Errorf("increment clicks: %w", er)
	}
	count, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("check affected rows: %w", err)
    }
    if count == 0 {
        return sql.ErrNoRows
    }
	return nil
}



// func (s *URLStore) saveUrls(filename string) error {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()

// 	jsonBytes, err := json.Marshal(s.urlDB)
// 	if err != nil {
// 		return fmt.Errorf("Error Parsing JSON")
// 	}

// 	// jsonStr := string(jsonBytes)
// 	file, err := os.CreateTemp(filepath.Dir(filename), "tmp.json")
// 	if err != nil {
// 		return fmt.Errorf("Error creating temporary file")
// 	}
// 	defer file.Close()
// 	defer os.Remove(file.Name())
// 	er := os.WriteFile(file.Name(), jsonBytes, 0644)
// 	if er != nil {
// 		return fmt.Errorf("Error in Writing to the File")
// 	}
// 	errors := os.Rename(file.Name(), filename)
// 	if errors != nil {
// 		return fmt.Errorf("Error in Renaming the File")
// 	}

// 	return nil

// }

// func loadUrls(filename string) error {
// 	data, err := os.ReadFile(filename)

// 	if err != nil {
// 		return fmt.Errorf("read Url File :%w", err)
// 	}

// 	jsonerr := json.Unmarshal(data, &urlStore.urlDB)
// 	if jsonerr != nil {
// 		return fmt.Errorf("Error in Unmarshaling Json")
// 	}
// 	return nil
// }

func main() {
	// generatedShortURL := createShortURL("https://example.com")
	// fmt.Println("Generated Short URL:", generatedShortURL)

	db, err := sql.Open("sqlite3", "./urls.db")
	if err != nil {
		fmt.Println("Error opening database:", err)
		return
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Println("Error connecting to database:", err)
		return
	}

	urlStore := &URLStore{db: db}

	http.HandleFunc("/shorten", shortUrlHandler(urlStore))
	http.HandleFunc("/redirect/", redirectHandler(urlStore))
	// loaderr:=loadUrls("urls.json")
	// if loaderr!=nil{
	// 	fmt.Println("Error Loading file: %w",loaderr)
	// }
	err2 := http.ListenAndServe(":8080", nil)
	if err2 != nil {
		fmt.Println("Error starting server:", err2)
	}

}
