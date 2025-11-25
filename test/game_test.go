package test

import (
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/iggy157/aiwolf-nlp-server-unlimited/core"
	"github.com/iggy157/aiwolf-nlp-server-unlimited/model"
	"github.com/joho/godotenv"
	"golang.org/x/exp/rand"
)

const WebSocketExternalHost = "0.0.0.0"

func TestGame(t *testing.T) {
	if _, exists := os.LookupEnv("GITHUB_ACTIONS"); !exists {
		err := godotenv.Load("../config/.env")
		if err != nil {
			t.Fatalf("Failed to load .env file: %v", err)
		}
	}

	config, err := model.LoadFromPath("../config/debug.yml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if _, exists := os.LookupEnv("GITHUB_ACTIONS"); exists {
		config.Server.WebSocket.Host = WebSocketExternalHost
	}
	go func() {
		server := core.NewServer(*config)
		server.Run()
	}()
	time.Sleep(5 * time.Second)

	u := url.URL{Scheme: "ws", Host: config.Server.WebSocket.Host + ":" + strconv.Itoa(config.Server.WebSocket.Port), Path: "/ws"}
	t.Logf("Connecting to %s", u.String())

	names := make([]string, config.Game.AgentCount)
	for i := range config.Game.AgentCount {
		const letterBytes = "abcdefghijklmnopqrstuvwxyz"
		b := make([]byte, 8)
		for i := range b {
			b[i] = letterBytes[rand.Intn(len(letterBytes))]
		}
		names[i] = string(b)
	}

	clients := make([]*DummyClient, config.Game.AgentCount)
	for i := range config.Game.AgentCount {
		client, err := NewDummyClient(u, names[i], t)
		if err != nil {
			t.Fatalf("Failed to create WebSocket client: %v", err)
		}
		clients[i] = client
		defer clients[i].Close()
	}

	for _, client := range clients {
		select {
		case <-client.done:
			t.Log("Connection closed")
		case <-time.After(5 * time.Minute):
			t.Fatalf("Timeout")
		}
	}

	time.Sleep(5 * time.Second)
	t.Log("Test completed successfully")
}

func TestManualGame(t *testing.T) {
	if _, exists := os.LookupEnv("GITHUB_ACTIONS"); !exists {
		err := godotenv.Load("../config/.env")
		if err != nil {
			t.Fatalf("Failed to load .env file: %v", err)
		}
	}

	config, err := model.LoadFromPath("../config/debug.yml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if _, exists := os.LookupEnv("GITHUB_ACTIONS"); exists {
		config.Server.WebSocket.Host = WebSocketExternalHost
		return
	}
	go func() {
		server := core.NewServer(*config)
		server.Run()
	}()
	time.Sleep(5 * time.Second)

	u := url.URL{Scheme: "ws", Host: config.Server.WebSocket.Host + ":" + strconv.Itoa(config.Server.WebSocket.Port), Path: "/ws"}
	t.Logf("Connecting to %s", u.String())

	names := make([]string, config.Game.AgentCount-1)
	for i := range config.Game.AgentCount - 1 {
		const letterBytes = "abcdefghijklmnopqrstuvwxyz"
		b := make([]byte, 8)
		for i := range b {
			b[i] = letterBytes[rand.Intn(len(letterBytes))]
		}
		names[i] = string(b)
	}

	clients := make([]*DummyClient, config.Game.AgentCount-1)
	for i := range config.Game.AgentCount - 1 {
		client, err := NewDummyClient(u, names[i], t)
		if err != nil {
			t.Fatalf("Failed to create WebSocket client: %v", err)
		}
		clients[i] = client
		defer clients[i].Close()
	}

	for _, client := range clients {
		<-client.done
		t.Log("Connection closed")
	}

	time.Sleep(5 * time.Second)
	t.Log("Test completed successfully")
}
