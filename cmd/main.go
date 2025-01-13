package main

import (
  "log"
  "net/http"

  "github.com/eliuttth-dev/go-url-shortener/internal/routes"
)

func main(){
  router := routes.SetupRouter()

  // Server
  serverAddr := ":3030"
  log.Printf("Starting server on %s\n", serverAddr)
  log.Fatal(http.ListenAndServe(serverAddr,router))
}
