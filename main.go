package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var db *pgxpool.Pool

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

	rows, err := db.Query(r.Context(), "SELECT id, original_url, clicks FROM links")
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	allLinks := make(map[string]*Link)
	for rows.Next() {
		var id string
		var link Link
		if err := rows.Scan(&id, &link.OriginalURL, &link.Clicks); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		allLinks[id] = &link
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "iteration error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	enc.Encode(allLinks)
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/stats/")

	var link Link
	err := db.QueryRow(r.Context(), "SELECT original_url, clicks FROM links WHERE id = $1", id).Scan(&link.OriginalURL, &link.Clicks)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
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

	_, err := db.Exec(r.Context(), "INSERT INTO links (id, original_url, clicks) VALUES ($1, $2, $3)", id, req.URL, 0)
	if err != nil {
		http.Error(w, "failed to save link", http.StatusInternalServerError)
		return
	}

	res := fmt.Sprintf("http://localhost:8085/%s", id)
	w.Write([]byte(res))
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[1:]

	var originalURL string
	err := db.QueryRow(r.Context(), "UPDATE links SET clicks = clicks + 1 WHERE id = $1 RETURNING original_url", id).Scan(&originalURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "database error", http.StatusInternalServerError)
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
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}

	ctx := context.Background()
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatalf("DATABASE_URL не задана в переменных окружения")
	}

	var err error
	db, err = pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("ошибка подключения к бд: %v\n", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatalf("База недоступна: %s\n", err.Error())
	}
	fmt.Println("Успешное подключение к БД")

	http.HandleFunc("/ping", pingHandler)
	http.HandleFunc("/shorten", shortenHandler)
	http.HandleFunc("/", redirectHandler)
	http.HandleFunc("/stats/", statsHandler)
	http.HandleFunc("/links", linksHandler)

	addr := fmt.Sprintf(":%s", port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Ошибка сервера: %v\n", err)
	}

}
