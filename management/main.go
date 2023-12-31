package main

import (
	"log"
	"net/http"

	"github.com/tanaka-takurou/serverless-image-board-go/management/controller"
)

func main() {
	http.HandleFunc("/", controller.HttpHandler)
	log.Println("Server listening on http://localhost:8080")
	log.Print(http.ListenAndServe(":8080", nil))
}
