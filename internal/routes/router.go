package routes

import(
  "github.com/gorilla/mux"
  "github.com/eliuttth-dev/go-url-shortener/internal/handlers"
)

func SetupRouter() *mux.Router {
  r := mux.NewRouter()

  shortenerHandler := handlers.NewURLShortenerHandler()

  // Routes
  r.HandleFunc("/generate", shortenerHandler.GenerateHandler).Methods("POST")
  r.HandleFunc("/{shortURL}", shortenerHandler.ResolveHandler).Methods("GET")

  return r
}
