package main

import (
  "log"
  "net/http"

  "go-url-shortener/internal/routes"
)

func main(){
  dbPath := "./urls.db"
  redisAddr := "localhost:6379"

  router, err := routes.SetupRouter(dbPath, redisAddr)
  if err != nil {
    log.Fatalf("Failed to set up router: %v", err)
  }

  // Server
  serverAddr := ":3030"
  log.Printf("Starting server on %s\n", serverAddr)
  log.Fatal(http.ListenAndServe(serverAddr,router))
}
