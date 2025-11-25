package service

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/iggy157/aiwolf-nlp-server-unlimited/model"
	"github.com/iggy157/aiwolf-nlp-server-unlimited/util"
	"github.com/gorilla/websocket"
)

type RealtimeBroadcaster struct {
	config   model.Config
	upgrader websocket.Upgrader
	clients  *sync.Map
}

func NewRealtimeBroadcaster(config model.Config) *RealtimeBroadcaster {
	return &RealtimeBroadcaster{
		config: config,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		clients: &sync.Map{},
	}
}

func (rb *RealtimeBroadcaster) Broadcast(packet model.BroadcastPacket) {
	var disconnectedClients []*websocket.Conn
	rb.clients.Range(func(key, value any) bool {
		client := key.(*websocket.Conn)
		if err := client.WriteJSON(packet); err != nil {
			slog.Warn("クライアントへのメッセージ送信に失敗しました", "error", err)
			disconnectedClients = append(disconnectedClients, client)
		}
		return true
	})
	for _, client := range disconnectedClients {
		client.Close()
		rb.clients.Delete(client)
	}
	slog.Info("リアルタイムブロードキャストを送信しました", "packet", packet)
}

func (rb *RealtimeBroadcaster) HandleConnections(w http.ResponseWriter, r *http.Request) {
	if rb.config.Server.Authentication.Enable {
		token := r.URL.Query().Get("token")
		if token != "" {
			if !util.IsValidReceiver(rb.config.Server.Authentication.Secret, token) {
				slog.Warn("トークンが無効です")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		} else {
			token = strings.ReplaceAll(r.Header.Get("Authorization"), "Bearer ", "")
			if !util.IsValidReceiver(rb.config.Server.Authentication.Secret, token) {
				slog.Warn("トークンが無効です")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
	}

	ws, err := rb.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("クライアントのアップグレードに失敗しました", "error", err)
		return
	}
	defer ws.Close()

	rb.clients.Store(ws, nil)
	defer rb.clients.Delete(ws)

	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			slog.Error("クライアントの読み込みに失敗しました", "error", err)
			break
		}
	}
}
