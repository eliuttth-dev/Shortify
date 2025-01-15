package handlers

import (
  "bytes"
  "database/sql"
  "encoding/json"
  "net/http"
  "net/http/httptest"
  "os"
  "testing"
  "time"
  "context"

  "github.com/redis/go-redis/v9"
  "github.com/gorilla/mux"
  _ "github.com/mattn/go-sqlite3"
)

// Initializes a temporary SQLite database for testing
func setupTestDB(t *testing.T) *sql.DB {
  t.Helper()

  // Create a temporary SQLite database for testing
  dbPath := "./test_urls.db"
  os.Remove(dbPath)

  db, err := sql.Open("sqlite3", dbPath)
  if err != nil {
    t.Fatalf("Failed to set up test database: %v", err)
  }

  createTableQuery := `
  CREATE TABLE IF NOT EXISTS urls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    short_url TEXT NOT NULL UNIQUE,
    original_url TEXT NOT NULL
  );`

  _, err = db.Exec(createTableQuery)
  if err != nil {
    t.Fatalf("Failed to create table: %v", err)
  }

  t.Cleanup(func() {
    db.Close()
    os.Remove(dbPath) 
  })

  return db
}

// Initializes a temporary Redis client for testing
func setupTestRedis(t *testing.T) *redis.Client{
  t.Helper()

  redisAddr := "localhost:6379"
  client := redis.NewClient(&redis.Options{
    Addr: redisAddr,
  })

  // Ensures Redis connectivity
  ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
  defer cancel()

  if err := client.FlushAll(ctx).Err(); err != nil {
    t.Fatalf("Failed to flush Redis: %v", err)
  }

  if err := client.Ping(ctx).Err(); err != nil {
    t.Fatalf("Failed to connect to Redis: %v", err)
  }

  t.Cleanup(func(){
    client.FlushAll(ctx)
    client.Close()
  })

  return client
}

// Test the GenerateHandler
func TestGenerateHandler(t *testing.T) {
  _ = setupTestDB(t)
  _ = setupTestRedis(t)

  handler, err := NewURLShortenerHandler("./test_urls.db", "localhost:6379")
  if err != nil {
    t.Fatalf("Failed to initialize handler: %v", err)
  }

  tests := []struct {
    name           string
    payload        string
    expectedStatus int
    expectedError  bool
  }{
    {"Valid URL", `{"original_url": "https://github.com/eliuttth-dev"}`, http.StatusOK, false},
    {"Empty URL", `{"original_url": ""}`, http.StatusBadRequest, true},
    {"Malformed JSON", `{"original_url":`, http.StatusBadRequest, true},
    {"Missing URL", `{}`, http.StatusBadRequest, true},
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      req := httptest.NewRequest("POST", "/generate", bytes.NewBuffer([]byte(tt.payload)))
      req.Header.Set("Content-Type", "application/json")
      w := httptest.NewRecorder()

      handler.GenerateHandler(w, req)

      resp := w.Result()
      defer resp.Body.Close()

      if resp.StatusCode != tt.expectedStatus {
        t.Errorf("Expected status %v, got %v", tt.expectedStatus, resp.StatusCode)
      }

      if !tt.expectedError {
        var body map[string]string
        if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
          t.Errorf("Failed to decode response: %v", err)
        }

        // Validate short URL
        shortURL, exists := body["short_url"]
        if !exists || shortURL == "" {
          t.Errorf("Expected a valid short_url, got %v", body)
        }
      }
    })
  }
}

// Test the ResolveHandler
func TestResolveHandler(t *testing.T) {
  _ = setupTestDB(t)
  _ = setupTestRedis(t)
  
  handler, err := NewURLShortenerHandler("./test_urls.db", "localhost:6379")
  if err != nil {
    t.Fatalf("Failed to initialize handler: %v", err)
  }

  // Prepopulate the database with a short URL
  originalURL := "https://github.com/eliuttth-dev"
  shortURL, err := handler.Shortener.GenerateShortURL(originalURL)
  if err != nil {
    t.Fatalf("Failed to prepopulate database: %v", err)
  }

  tests := []struct {
    name           string
    shortURL       string
    expectedStatus int
    expectedURL    string
  }{
    {"Valid Short URL", shortURL, http.StatusFound, originalURL},
    {"Non-existent Short URL", "nonexistent", http.StatusNotFound, ""},
    {"Empty Short URL", "", http.StatusNotFound, ""},
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      req := httptest.NewRequest("GET", "/"+tt.shortURL, nil)
      w := httptest.NewRecorder()

      router := mux.NewRouter()
      router.HandleFunc("/{shortURL}", handler.ResolveHandler)
      router.ServeHTTP(w, req)

      resp := w.Result()
      defer resp.Body.Close()

      if resp.StatusCode != tt.expectedStatus {
        t.Errorf("Expected status %v, got %v", tt.expectedStatus, resp.StatusCode)
      }

      if tt.expectedStatus == http.StatusFound {
        location := resp.Header.Get("Location")
        if location != tt.expectedURL {
          t.Errorf("Expected redirect to %v, got %v", tt.expectedURL, location)
        }
      }
    })
  }
}

