package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func catchAll(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	log.Println("================================")
	log.Println("METHOD:", r.Method)
	log.Println("PATH:", r.URL.Path)
	log.Println("QUERY:", r.URL.RawQuery)
	log.Println("BODY:", string(body))
	log.Println("================================")

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func main() {
	// Health endpoint
	http.HandleFunc("/health", healthHandler)

	// Catch all ADMS requests
	http.HandleFunc("/", catchAll)

	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
