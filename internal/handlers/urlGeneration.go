package handlers

import (
  "context"
  "database/sql"
  "encoding/json"
  "net/http"
  "sync"
  "log"
  "fmt"
  "time"
  "strings"
  "github.com/redis/go-redis/v9"
  "github.com/gorilla/mux"
  _ "github.com/mattn/go-sqlite3"
)

// Manage the URL shortening and resolution
type URLShortener struct {
  db *sql.DB
  cache *redis.Client
  mu sync.Mutex
}

// Handles HTTP request related to URL shortening and resolution
type URLShortenerHandler struct {
  Shortener *URLShortener
}

//  Initializes the URLShortener instance, setting up the SQLite database
//  and Redis client. It also ensures the necessary database table exists
//
//  Parameters:
//    - dbPath: The path to the SQlite database file
//    - redisAddr: the address of the Redis server
//
//  Returns:
//    - A pointer to the URLShortener instance
//    - An error if the database or Redis initialization fails
func NewURLGeneration(dbPath string, redisAddr string) (*URLShortener, error) {
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

  // Redis Client
  cache := redis.NewClient(&redis.Options{
    Addr: redisAddr,
  })

  // Ping Redis to ensure connectivity
  if err := cache.Ping(context.Background()).Err(); err != nil {
    return nil, fmt.Errorf("Failed to connect to [Redis]: %w", err)
  }

  return &URLShortener{
    db: db,
    cache: cache,
  }, nil
}

// CHANGE THIS TO A SEPARATE UTIL FILE
func isValidCustomURL(customURL string) bool {
  const validChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_"
  for _, char := range customURL {
    if !strings.ConainsRune(validChars, char) {
      return false
    }
  }
  return true
}

// Generates a unique short URL
func (us *URLShortener) GenerateShortURL(originalURL string, customShortURL string) (string, error) {
  us.mu.Lock()
  defer us.mu.Unlock()

  if customShortURL != "" {
    if !isValidCustomURL(customShortURL) {
      return "", errors.New("Invalid characters in custom short URL")
    }
    
    // Check if custom short URL already exits
    var exists bool
    query := `SELECT EXISTS(SELECT 1 FROM urls WHERE short_url = ?)`
    err := us.db.QueryRow(query, customShortURL).Scan(&exists)
    if err != nil {
      return "", fmt.Errorf("Database error: %v", err)
    }
    if exists {
      return "". fmt.Errorf("Custom short URL already exists")
    }

    // Insert the ustom short URL into the database
    insertQuery := `INSERT INTO urls (short_url, original_url) VALUES (?, ?)`
    _, err = us.db.Exec(insertQuery, customShortURL, originalURL)
    if err != nil {
      return "", fmt.Errorf("Failed to store custom short URL: %v", err)
    }

    return customShortURL, nil
  }

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
  ctx := context.Background()

  // Check Redis cache first
  cachedURL, err := us.cache.Get(ctx, shortURL).Result()
  if err == nil {
    log.Printf("[Redis] Cache hit: %s --> %s", shortURL, cachedURL)
    return cachedURL, true
  } else if err != redis.Nil { // redis.Nil means key does not exist
    log.Printf("[Redis] Cache error for %s: %v", shortURL, err)
  }

  log.Printf("[Redis] Cache miss: %s", shortURL)

  // Fallback to database lookup
  us.mu.Lock()
  defer us.mu.Unlock()

  query := `SELECT original_url FROM urls WHERE short_url = ?`
  var originalURL string
  err = us.db.QueryRow(query, shortURL).Scan(&originalURL)
  if err != nil {
    log.Printf("[DB] Short URL not found: %s", shortURL)
    return "", false
  }

  // Add result to Redis cache
  err = us.cache.Set(ctx, shortURL, originalURL, 24*time.Hour).Err()
  if err != nil {
    log.Printf("[Redis] Failed to cache short URL: %s -> %s, error: %v", shortURL, originalURL, err)
  } else {
    log.Printf("[Redis] Cached short URL: %s -> %s", shortURL, originalURL)
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
func NewURLShortenerHandler(dbPath, redisAddr string) (*URLShortenerHandler, error) {
  shortener, err := NewURLGeneration(dbPath, redisAddr)
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
    CustomShortURL string `json:"custom_short_url,omitempty"`
  }
  
  // Decode JSON Body
  if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
    http.Error(w, "Invalid request body: Please provide a valid JSON payload", http.StatusBadRequest)
    return
  }

  // Check for missing or empty original_url
  if body.OriginalURL == "" {
    http.Error(w, "Missing required field: 'original_url' cannot be empty", http.StatusBadRequest)
    return
  }

  // Generate the short URL
  shortURL, err := h.Shortener.GenerateShortURL(body.OriginalURL, body.CustomShortURL)
  if err != nil {
    http.Error(w, fmt.Sprintf("Failed to generate short URL: %v", err), http.StatusBadRequest)
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
  
  // Check if short URL is empty
  if shortURL == "" {
    http.Error(w, "Invalid request: 'shortURL' cannot be empty", http.StatusBadRequest)
    return
  }

  // Resolve the short URL
  originalURL, exists := h.Shortener.ResolveShortURL(shortURL)
  if !exists {
    http.Error(w, "Short URL not found: No record exists for the given 'shortURL'", http.StatusNotFound)
    return
  }

  // Redirect to the original URL
  http.Redirect(w, r, originalURL, http.StatusFound)
}

