package main

import (
	"log"
	"net/http"
	"./controller"
)

func main() {
	http.HandleFunc("/", controller.HttpHandler)
	log.Println("Server listening on http://localhost:8080")
	log.Print(http.ListenAndServe(":8080", nil))
}
