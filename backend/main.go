package main

import (
	"bingo/bingo"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	path := os.Getenv("BINGO_DATA")
	if path == "" {
		path = "bingo.json"
	}
	if err := bingo.InitData(path); err != nil {
		log.Fatal(err)
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	adminConfig, err := loadAdminConfig(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	app := newServer(os.Getenv("SECURE_COOKIE") == "true")
	app.admin = newAdminService(adminConfig, path)
	server := &http.Server{Addr: addr, Handler: app, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("SMW Bingo listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
