package main

import (
  "log"
  "net/http"

  "go-url-shortener/internal/routes"
)

func main(){
  dbPath := "../urls.db"
  router, err := routes.SetupRouter(dbPath)
  if err != nil {
    log.Fatalf("Failed to set up router: %v", err)
  }

  // Server
  serverAddr := ":3030"
  log.Printf("Starting server on %s\n", serverAddr)
  log.Fatal(http.ListenAndServe(serverAddr,router))
}
