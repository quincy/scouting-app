package main

import (
    "fmt"
    "net/http"
)

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "OK")
}

func main() {
    http.HandleFunc("/healthcheck", healthCheckHandler)
    http.ListenAndServe(":8080", nil)
}