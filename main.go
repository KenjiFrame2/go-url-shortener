package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

var (
	storage = make(map[string]*Link)
	mu      sync.RWMutex
)

type Link struct {
	OriginalURL string `json:"original_url"`
	Clicks      int    `json:"clicks"`
}

type ShortenRequest struct {
	URL string `json:"url"`
}

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func linksHandler(w http.ResponseWriter, r *http.Request) {

	mu.RLock()
	copyMap := make(map[string]*Link, len(storage))
	for k, v := range storage {
		copyMap[k] = v
	}
	mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	enc.Encode(copyMap)
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/stats/")
	mu.RLock()
	link, exists := storage[id]
	mu.RUnlock()

	if !exists {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(link)
}

func shortenHandler(w http.ResponseWriter, r *http.Request) {
	var req ShortenRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	id := generateID()
	mu.Lock()
	storage[id] = &Link{
		OriginalURL: req.URL,
		Clicks:      0,
	}
	mu.Unlock()
	res := fmt.Sprintf("http://localhost:8085/%s", id)
	w.Write([]byte(res))
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[1:]
	mu.Lock()
	defer mu.Unlock()
	link, exists := storage[id]

	if !exists {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	link.Clicks++

	http.Redirect(w, r, link.OriginalURL, http.StatusTemporaryRedirect)
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		w.Write([]byte("pong"))
	} else {
		str := fmt.Sprintf("hello, %s", name)
		w.Write([]byte(str))
	}
}

func main() {
	http.HandleFunc("/ping", pingHandler)
	http.HandleFunc("/shorten", shortenHandler)
	http.HandleFunc("/", redirectHandler)
	http.HandleFunc("/stats/", statsHandler)
	http.HandleFunc("/links", linksHandler)
	http.ListenAndServe(":8085", nil)

}
