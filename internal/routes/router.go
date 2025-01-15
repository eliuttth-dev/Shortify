package routes

import(
  "github.com/gorilla/mux"
  "go-url-shortener/internal/handlers"
)

func SetupRouter(dbPath, redisAddr string) (*mux.Router, error) {
  r := mux.NewRouter()

  shortenerHandler, err := handlers.NewURLShortenerHandler(dbPath, redisAddr)
  if err != nil {
    return nil, err
  }

  // Routes
  r.HandleFunc("/generate", shortenerHandler.GenerateHandler).Methods("POST")
  r.HandleFunc("/{shortURL}", shortenerHandler.ResolveHandler).Methods("GET")

  return r, nil
}
