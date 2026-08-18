package web

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func (s *Server) chatWS(w http.ResponseWriter, r *http.Request) {
	ticketID := r.URL.Query().Get("ticket")
	wantChannel := r.URL.Query().Get("channelId")
	t, err := s.svc.AuthorizeChat(ticketID, wantChannel)
	if err != nil {
		writeErr(w, err)
		return
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(req *http.Request) bool {
			if s.corsOrigin == "" {
				return true
			}
			return req.Header.Get("Origin") == s.corsOrigin
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	out := s.hub.Subscribe(t.ChannelID)
	defer s.hub.Unsubscribe(t.ChannelID, out)

	go func() {
		defer conn.Close()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(data, &msg) != nil {
				continue
			}
			if msg.Type == "typing" && !t.ReadOnly {
				s.svc.BroadcastTyping(t.ChannelID, t.Sub)
			}
		}
	}()

	for payload := range out {
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			return
		}
	}
}
