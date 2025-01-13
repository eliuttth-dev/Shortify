package handlers

import (
  "encoding/json"
  "net/http"
  "sync"

  "github.com/gorilla/mux"
)

type URLShortener struct {
  mapping map[string]string
  counter int64
  mu      sync.Mutex
}

func NewURLGeneration() *URLShortener {
  return &URLShortener{
    mapping: make(map[string]string),
    counter: 1,
  }
}

// Generates a unique short URL
func (us *URLShortener) GenerateShortURL(originalURL string) string {
  us.mu.Lock()
  defer us.mu.Unlock()

  uniqueID := us.counter
  us.counter++

  shortURL := encodeBase62(uniqueID)
  us.mapping[shortURL] = originalURL

  return shortURL
}

// Resolves a short URL back to the original URL
func (us *URLShortener) ResolveShortURL(shortURL string) (string, bool) {
  us.mu.Lock()
  defer us.mu.Unlock()

  originalURL, exists := us.mapping[shortURL]
  return originalURL, exists
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

func NewURLShortenerHandler() *URLShortenerHandler {
  return &URLShortenerHandler{
    Shortener: NewURLGeneration(),
  }
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

  shortURL := h.Shortener.GenerateShortURL(body.OriginalURL)

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

