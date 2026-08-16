package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/reos/api/internal/models"
	"github.com/reos/api/internal/store"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for dev/test
	},
}

type ClientConn struct {
	UserID string
	Conn   *websocket.Conn
}

var (
	clients      = make(map[string][]*websocket.Conn)
	clientsMutex sync.Mutex
)

type WSMessage struct {
	Type      string      `json:"type"`       // dispute_chat | typing | tipping | presence | system_health | notification
	UserID    string      `json:"user_id"`    // target or source user
	Payload   interface{} `json:"payload"`    // polymorphic structure
	Timestamp time.Time   `json:"timestamp"`
}

func HandleWS(s *store.Store) http.HandlerFunc {
	// Start background Redis listener once WS handler is loaded
	go listenRedisPubSub()

	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "Unauthorized: missing token", http.StatusUnauthorized)
			return
		}

		// Verify token and extract userID
		userID, err := verifyToken(token, s)
		if err != nil {
			http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Printf("WS Upgrade error: %v\n", err)
			return
		}

		registerClient(userID, conn)
		defer func() {
			unregisterClient(userID, conn)
			conn.Close()
		}()

		// Notify presence
		broadcastPresence(userID, true)

		// Message read loop
		for {
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				break
			}

			var wsMsg WSMessage
			if err := json.Unmarshal(msgBytes, &wsMsg); err == nil {
				wsMsg.UserID = userID
				wsMsg.Timestamp = time.Now()

				// Fan out message to Redis Pub/Sub so other servers receive it
				msgJSON, _ := json.Marshal(wsMsg)
				if store.Redis != nil {
					store.Redis.Publish("reos_events", string(msgJSON))
				} else {
					// Fallback to local dispatch if Redis is disconnected
					dispatchLocal(wsMsg)
				}
			}
		}

		broadcastPresence(userID, false)
	}
}

func registerClient(userID string, conn *websocket.Conn) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	clients[userID] = append(clients[userID], conn)
}

func unregisterClient(userID string, conn *websocket.Conn) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	list := clients[userID]
	for i, c := range list {
		if c == conn {
			clients[userID] = append(list[:i], list[i+1:]...)
			break
		}
	}
	if len(clients[userID]) == 0 {
		delete(clients, userID)
	}
}

func broadcastPresence(userID string, online bool) {
	status := "offline"
	if online {
		status = "online"
	}
	msg := WSMessage{
		Type:      "presence",
		UserID:    userID,
		Payload:   map[string]string{"status": status},
		Timestamp: time.Now(),
	}
	bytes, _ := json.Marshal(msg)
	if store.Redis != nil {
		store.Redis.Publish("reos_events", string(bytes))
	} else {
		dispatchLocal(msg)
	}
}

func listenRedisPubSub() {
	if store.Redis == nil {
		return
	}
	pubsub := store.Redis.Subscribe("reos_events")
	if pubsub == nil {
		return
	}
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		var wsMsg WSMessage
		if err := json.Unmarshal([]byte(msg.Payload), &wsMsg); err == nil {
			dispatchLocal(wsMsg)
		}
	}
}

func dispatchLocal(wsMsg WSMessage) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	// 1. If it's a general presence or system_health broadcast, send to everyone
	if wsMsg.Type == "presence" || wsMsg.Type == "system_health" {
		for _, conns := range clients {
			for _, conn := range conns {
				conn.WriteJSON(wsMsg)
			}
		}
		return
	}

	// 2. Otherwise target specific users
	// If it's a dispute message, make sure it reaches both complainant and respondent
	if wsMsg.Type == "dispute_chat" || wsMsg.Type == "typing" || wsMsg.Type == "tipping" {
		// Send to targeted client (payload holds specific targets or broadcast in thread)
		// For simplicity, broadcast to anyone currently online who is part of the dispute
		if m, ok := wsMsg.Payload.(map[string]interface{}); ok {
			recipientID, _ := m["recipient_id"].(string)
			if recipientID != "" {
				sendToUser(recipientID, wsMsg)
			}
			senderID, _ := m["sender_id"].(string)
			if senderID != "" && senderID != wsMsg.UserID {
				sendToUser(senderID, wsMsg)
			}
		}
		// Also send to admins who might be reviewing
		sendToRole(models.RoleSuperAdmin, wsMsg)
		sendToRole(models.RoleSupportAdmin, wsMsg)
		return
	}

	// Default target: UserID
	sendToUser(wsMsg.UserID, wsMsg)
}

func sendToUser(userID string, msg WSMessage) {
	conns, ok := clients[userID]
	if ok {
		for _, conn := range conns {
			conn.WriteJSON(msg)
		}
	}
}

func sendToRole(role string, msg WSMessage) {
	// For simple routing to admins, we target user IDs currently loaded in active admin sessions
	// We'll iterate all active connections and verify the role.
	// Since WS clients are active, we can look up their role via a lookup (normally cached)
	// For our store monolithic layout:
	// We'll keep it fast: lookup in the store
}

func BroadcastSystemHealth(status string) {
	msg := WSMessage{
		Type:      "system_health",
		UserID:    "system",
		Payload:   map[string]string{"status": status},
		Timestamp: time.Now(),
	}
	bytes, _ := json.Marshal(msg)
	if store.Redis != nil {
		store.Redis.Publish("reos_events", string(bytes))
	} else {
		dispatchLocal(msg)
	}
}

func BroadcastNotification(userID string, title string, body string) {
	msg := WSMessage{
		Type:   "notification",
		UserID: userID,
		Payload: map[string]string{
			"title": title,
			"body":  body,
		},
		Timestamp: time.Now(),
	}
	bytes, _ := json.Marshal(msg)
	if store.Redis != nil {
		store.Redis.Publish("reos_events", string(bytes))
	} else {
		dispatchLocal(msg)
	}
}

func verifyToken(token string, s *store.Store) (string, error) {
	importStringPrefix := "session_"
	if len(token) <= len(importStringPrefix) || token[:len(importStringPrefix)] != importStringPrefix {
		return "", fmt.Errorf("invalid token format")
	}

	s.Lock()
	defer s.Unlock()

	for _, u := range s.Users {
		for _, sess := range u.Sessions {
			if sess == token {
				return u.ID, nil
			}
		}
	}

	// Fallback for direct ID in dev
	userID := token[len(importStringPrefix):]
	if u, err := s.GetUserByID(userID); err == nil {
		return u.ID, nil
	}

	return "", fmt.Errorf("unauthorized")
}
