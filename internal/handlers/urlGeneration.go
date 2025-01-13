package handlers

import (
  "database/sql"
  "encoding/json"
  "net/http"
  "sync"

  "github.com/gorilla/mux"
  _ "github.com/mattn/go-sqlite3"
)


type URLShortener struct {
  db *sql.DB
  mu sync.Mutex
}

func NewURLGeneration(dbPath string) (*URLShortener, error) {
  db, err := sql.Open("sqlite3", dbPath)
  if err != nil {
    return nil, err
  }

  // Create table if it doesn't exist
  createTableQuery := `
  CREATE TABLE IF NOT EXISTS urls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    short_url TEXT NOT NULL UNIQUE,
    original_url TEXT NOT NULL
  );`
  _, err = db.Exec(createTableQuery)
  if err != nil {
    return nil, err
  }

  return &URLShortener{
    db: db,
  }, nil
}
// Generates a unique short URL
func (us *URLShortener) GenerateShortURL(originalURL string) (string, error) {
  us.mu.Lock()
  defer us.mu.Unlock()

  // Generate a unique ID for the short URL
  var id int64
  query := `SELECT COALESCE(MAX(id), 0) + 1 FROM urls`
  err := us.db.QueryRow(query).Scan(&id)
  if err != nil {
      return "", err
  }

  shortURL := encodeBase62(id)

  // Insert the record with the short URL and original URL
  insertQuery := `INSERT INTO urls (id, short_url, original_url) VALUES (?, ?, ?)`
  _, err = us.db.Exec(insertQuery, id, shortURL, originalURL)
  if err != nil {
      return "", err
  }

  return shortURL, nil
}

// Resolves a short URL back to the original URL
func (us *URLShortener) ResolveShortURL(shortURL string) (string, bool) {
  us.mu.Lock()
  defer us.mu.Unlock()

  query := `SELECT original_url FROM urls WHERE short_url = ?`
  var originalURL string
  err := us.db.QueryRow(query, shortURL).Scan(&originalURL)
  if err != nil {
    return "", false
  }

  return originalURL, true
}

// Converts a numeric ID into a Base62 String
func encodeBase62(num int64) string {
  const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
  base := int64(len(charset))
  result := []byte{}

  for num > 0 {
    rem := num % base
    result = append([]byte{charset[rem]}, result...)
    num /= base
  }

  return string(result)
}

// Handles request related to URL shortening
type URLShortenerHandler struct {
  Shortener *URLShortener
}

func NewURLShortenerHandler(dbPath string) (*URLShortenerHandler, error) {
  shortener, err := NewURLGeneration(dbPath)
  if err != nil {
    return nil, err
  }

  return &URLShortenerHandler{
    Shortener: shortener,
  }, nil
}

// Handles request to generate short URL
func (h *URLShortenerHandler) GenerateHandler(w http.ResponseWriter, r *http.Request) {
  var body struct {
    OriginalURL string `json:"original_url"`
  }

  if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
    http.Error(w, "Invalid request body", http.StatusBadRequest)
    return
  }

  if body.OriginalURL == "" {
    http.Error(w, "original_url is required", http.StatusBadRequest)
    return
  }

  // Generate the short URL and handle any errors
  shortURL, err := h.Shortener.GenerateShortURL(body.OriginalURL)
  if err != nil {
    http.Error(w, "Failed to generate short URL: " +err.Error(), http.StatusInternalServerError)
    return
  }

  response := map[string]string{
    "short_url": shortURL,
  }

  w.Header().Set("Content-Type", "application/json")
  json.NewEncoder(w).Encode(response)
}
// Handles request to resolve short URL
func (h *URLShortenerHandler) ResolveHandler(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  shortURL := vars["shortURL"]


  originalURL, exists := h.Shortener.ResolveShortURL(shortURL)

  if !exists {
    http.Error(w, "Short URL not found", http.StatusNotFound)
    return
  }

  http.Redirect(w, r, originalURL, http.StatusFound)
}

