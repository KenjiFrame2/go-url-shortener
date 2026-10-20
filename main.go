package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

var (
	storage = make(map[string]string)
	mu      sync.RWMutex
)

type ShortenRequest struct {
	URL string `json:"url"`
}

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func shortenHandler(w http.ResponseWriter, r *http.Request) {
	var req ShortenRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	id := generateID()
	mu.Lock()
	storage[id] = req.URL
	mu.Unlock()
	res := fmt.Sprintf("http://localhost:8085/%s", id)
	w.Write([]byte(res))
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[1:]
	mu.RLock()
	originalURL, exists := storage[id]
	mu.RUnlock()
	if !exists {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	http.Redirect(w, r, originalURL, http.StatusTemporaryRedirect)
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
	http.ListenAndServe(":8085", nil)

}
