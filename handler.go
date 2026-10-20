package main

import (
	"encoding/json"
	"net/http"
)

func redirectHandler(filename string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/redirect/"):]
		originalURL, err := getOriginalURL(id)
		if err != nil {
			http.Error(w, "URL not found", http.StatusNotFound)
		return
	}
	incErr:=urlStore.urlIncrementClicks(id)
	if incErr !=nil{
		http.Error(w, "Not Incremented", http.StatusInternalServerError)
		return
	}
	er := urlStore.saveUrls(filename)
	if er!=nil{
		http.Error(w, "Error in Saving Urls", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, originalURL, http.StatusFound)

}
}

func shortUrlHandler(filename string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
	var Data struct {
		URL string `json:"url"`
	}

	err := json.NewDecoder(r.Body).Decode(&Data)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	err2 := validateURL(Data.URL)
	if err2 != nil {
		http.Error(w, "Provide a valid http or https URL",
			http.StatusBadRequest)
		return
	}

	shortURL := urlStore.createShortURL(Data.URL)
	if err := urlStore.saveUrls(filename); err != nil {
		http.Error(w, "Error saving URLs", http.StatusInternalServerError)
		return
	}
	response := struct {
		ShortURL string `json:"short_url"`
	}{
		ShortURL: shortURL.ShortURL,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
}