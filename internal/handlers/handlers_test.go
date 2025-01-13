package handlers

import (
  "bytes"
  "encoding/json"
  "net/http"
  "net/http/httptest"
  "testing"

  "github.com/gorilla/mux"
)

func TestGenerateHandler(t *testing.T) {
  handler := NewURLShortenerHandler()

  // Mock request
  payload := `{"original_url": "https://www.youtube.com/watch?v=n__NrG-QGb4"}`
  req := httptest.NewRequest("POST", "/generate", bytes.NewBuffer([]byte(payload)))
  req.Header.Set("Content-Type", "application/json")
  w := httptest.NewRecorder()

  handler.GenerateHandler(w, req)

  resp := w.Result()
  defer resp.Body.Close()

  if resp.StatusCode != http.StatusOK {
    t.Fatalf("expected status OK, got %v", resp.Status)
  }

  var body map[string]string
  if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
    t.Fatalf("failed to decode response: %v", err)
  }

  shortURL, exists := body["short_url"]
  if !exists || shortURL == "" {
    t.Fatalf("expected a short_url in response, got %v", body)
  }
}

func TestResolveHandler(t *testing.T) {
  handler := NewURLShortenerHandler()
  handler.Shortener.mapping["a"] = "https://www.youtube.com/watch?v=n__NrG-QGb4"

  // Mock request
  req := httptest.NewRequest("GET", "/resolve/a", nil)
  w := httptest.NewRecorder()

  // Use mux router for path variables
  router := mux.NewRouter()
  router.HandleFunc("/resolve/{shortURL}", handler.ResolveHandler)
  router.ServeHTTP(w, req)

  // Validate response
  resp := w.Result()
  defer resp.Body.Close()

  if resp.StatusCode != http.StatusFound {
    t.Fatalf("expected status Found, got %v", resp.Status)
  }

  if location := resp.Header.Get("Location"); location != "https://www.youtube.com/watch?v=n__NrG-QGb4" {
    t.Fatalf("expected redirect to https://www.youtube.com/watch?v=n__NrG-QGb4, got %v", location)
  }
}

