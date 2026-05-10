package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
    "github.com/gorilla/mux"
)

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "OK")
}

func main() {
    router := mux.NewRouter()
    router.HandleFunc("/healthcheck", healthCheckHandler).Methods("GET")

    srv := &http.Server{
        Addr:    ":8080",
        Handler: router,
    }

    go func() {
        if err := srv.ListenAndServe(); err != nil {
            log.Fatalf("Server ListenAndServe: %v", err)
        }
    }()

    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
    fmt.Println("Waiting for SIGINT or SIGTERM")
    <-sigs
    ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
    defer cancel()

    err := srv.Shutdown(ctx)
    if err != nil {
        log.Fatalf("Server Shutdown: %v", err)
    }
    fmt.Println("Server gracefully stopped")
}