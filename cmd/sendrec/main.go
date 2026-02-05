package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/sendrec/sendrec/internal/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := server.New()

	log.Printf("sendrec listening on :%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), srv); err != nil {
		log.Fatal(err)
	}
}
