package main

import (
	"log"
	"os"
	"time"

	dreego "github.com/dreego-stack/dreego/core"
	ws "github.com/dreego-stack/plugin-websocket"
)

func main() {
	app := dreego.New()
	if err := ws.Register(app, ws.Options{Path: "/ws"}); err != nil {
		log.Fatal(err)
	}
	go broadcastLoop()
	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}
	if err := app.Listen(addr); err != nil {
		log.Fatal(err)
	}
}

func broadcastLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		ws.HubInstance().Broadcast([]byte("ping"))
	}
}