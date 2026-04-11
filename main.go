package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed public/*
var content embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	id          string
	ip          string
	status      string
	controlConn *websocket.Conn
	mediaConn   *websocket.Conn
	writeMu     sync.Mutex
}

var (
	// rooms 按照 roomID 存储所有频道的客户端
	rooms      = make(map[string]map[string]*Client)
	roomsMutex sync.Mutex
	// 全局配置实例
	appConfig *Config
)

// UserInfo 表示房间内的单个用户信息
type UserInfo struct {
	ID     string `json:"id"`
	IP     string `json:"ip"`
	Status string `json:"status"`
}

// RoomInfo 存储下发给客户端的房间统计信息
type RoomInfo struct {
	Type  string     `json:"type"`
	Count int        `json:"count"`
	Users []UserInfo `json:"users"`
}

func broadcastRoomInfo(roomID string) {
	roomsMutex.Lock()
	clientsMap, ok := rooms[roomID]
	if !ok {
		roomsMutex.Unlock()
		return
	}

	recipients := make([]*Client, 0, len(clientsMap))
	users := make([]UserInfo, 0, len(clientsMap))
	for _, c := range clientsMap {
		if c.controlConn == nil {
			continue
		}
		recipients = append(recipients, c)
		users = append(users, UserInfo{
			ID:     c.id,
			IP:     c.ip,
			Status: c.status,
		})
	}
	roomsMutex.Unlock()

	info := RoomInfo{
		Type:  "room_info",
		Count: len(users),
		Users: users,
	}

	msg, _ := json.Marshal(info)

	for _, c := range recipients {
		_ = writeMessage(c, c.controlConn, websocket.TextMessage, msg)
	}
}

func writeMessage(client *Client, conn *websocket.Conn, messageType int, payload []byte) error {
	if client == nil || conn == nil {
		return nil
	}
	client.writeMu.Lock()
	defer client.writeMu.Unlock()
	return conn.WriteMessage(messageType, payload)
}

func getRoomID(r *http.Request) string {
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		roomID = "default"
	}
	return roomID
}

func getClientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip = forwarded
	}
	return ip
}

func countActiveClients(clientsMap map[string]*Client) int {
	count := 0
	for _, client := range clientsMap {
		if client.controlConn != nil {
			count++
		}
	}
	return count
}

func registerControlClient(roomID, clientID, ip string, conn *websocket.Conn) (*Client, *websocket.Conn, *websocket.Conn, bool) {
	var oldControlConn *websocket.Conn
	var oldMediaConn *websocket.Conn

	roomsMutex.Lock()
	if rooms[roomID] == nil {
		rooms[roomID] = make(map[string]*Client)
	}

	client := rooms[roomID][clientID]
	if client == nil {
		if appConfig.Mode == "walkie-talkie" && countActiveClients(rooms[roomID]) >= 2 {
			roomsMutex.Unlock()
			return nil, nil, nil, false
		}
		client = &Client{id: clientID, ip: ip, status: "就绪"}
		rooms[roomID][clientID] = client
	} else {
		if client.controlConn == nil && appConfig.Mode == "walkie-talkie" && countActiveClients(rooms[roomID]) >= 2 {
			roomsMutex.Unlock()
			return nil, nil, nil, false
		}
		oldControlConn = client.controlConn
		oldMediaConn = client.mediaConn
	}

	client.ip = ip
	client.status = "就绪"
	client.controlConn = conn
	client.mediaConn = nil
	roomsMutex.Unlock()

	return client, oldControlConn, oldMediaConn, true
}

func registerMediaClient(roomID, clientID string, conn *websocket.Conn) (*Client, *websocket.Conn, bool) {
	var oldMediaConn *websocket.Conn

	roomsMutex.Lock()
	defer roomsMutex.Unlock()

	clientsMap := rooms[roomID]
	if clientsMap == nil {
		return nil, nil, false
	}

	client := clientsMap[clientID]
	if client == nil || client.controlConn == nil {
		return nil, nil, false
	}

	oldMediaConn = client.mediaConn
	client.mediaConn = conn
	return client, oldMediaConn, true
}

func updateClientStatus(roomID, clientID, status string) bool {
	roomsMutex.Lock()
	defer roomsMutex.Unlock()

	clientsMap := rooms[roomID]
	if clientsMap == nil {
		return false
	}

	client := clientsMap[clientID]
	if client == nil || client.controlConn == nil {
		return false
	}

	client.status = status
	return true
}

func forwardControlSignal(roomID, senderID, messageType string) {
	roomsMutex.Lock()
	clientsMap := rooms[roomID]
	if clientsMap == nil {
		roomsMutex.Unlock()
		return
	}

	sender := clientsMap[senderID]
	if sender == nil || sender.controlConn == nil {
		roomsMutex.Unlock()
		return
	}

	recipients := make([]*Client, 0, len(clientsMap))
	for clientID, client := range clientsMap {
		if clientID == senderID || client.controlConn == nil {
			continue
		}
		recipients = append(recipients, client)
	}
	fromIP := sender.ip
	roomsMutex.Unlock()

	responseBytes, _ := json.Marshal(map[string]string{
		"type":   messageType,
		"fromIP": fromIP,
	})

	for _, client := range recipients {
		_ = writeMessage(client, client.controlConn, websocket.TextMessage, responseBytes)
	}
}

func forwardWebRTCSignal(roomID, senderID string, payload []byte) {
	roomsMutex.Lock()
	clientsMap := rooms[roomID]
	if clientsMap == nil {
		roomsMutex.Unlock()
		return
	}

	sender := clientsMap[senderID]
	if sender == nil || sender.controlConn == nil {
		roomsMutex.Unlock()
		return
	}

	// 解析出 targetID
	var msgData struct {
		TargetID string `json:"targetID"`
	}
	if err := json.Unmarshal(payload, &msgData); err != nil {
		roomsMutex.Unlock()
		return
	}

	// 为了让接收方知道是谁发来的，我们需要注入 senderID
	var genericMap map[string]interface{}
	if err := json.Unmarshal(payload, &genericMap); err == nil {
		genericMap["fromID"] = senderID
		payload, _ = json.Marshal(genericMap)
	}

	var targetClient *Client
	if msgData.TargetID != "" {
		targetClient = clientsMap[msgData.TargetID]
	}

	var recipients []*Client
	if targetClient != nil && targetClient.controlConn != nil {
		recipients = append(recipients, targetClient)
	} else if msgData.TargetID == "" {
		// Broadcast if no targetID
		for clientID, client := range clientsMap {
			if clientID == senderID || client.controlConn == nil {
				continue
			}
			recipients = append(recipients, client)
		}
	}
	roomsMutex.Unlock()

	for _, client := range recipients {
		_ = writeMessage(client, client.controlConn, websocket.TextMessage, payload)
	}
}

func forwardMediaMessage(roomID, senderID string, messageType int, payload []byte) {
	roomsMutex.Lock()
	clientsMap := rooms[roomID]
	if clientsMap == nil {
		roomsMutex.Unlock()
		return
	}

	sender := clientsMap[senderID]
	if sender == nil || sender.controlConn == nil {
		roomsMutex.Unlock()
		return
	}

	recipients := make([]*Client, 0, len(clientsMap))
	for clientID, client := range clientsMap {
		if clientID == senderID || client.mediaConn == nil {
			continue
		}
		recipients = append(recipients, client)
	}
	roomsMutex.Unlock()

	for _, client := range recipients {
		_ = writeMessage(client, client.mediaConn, messageType, payload)
	}
}

func removeControlClient(roomID, clientID string, conn *websocket.Conn) {
	var mediaConn *websocket.Conn
	removed := false

	roomsMutex.Lock()
	clientsMap := rooms[roomID]
	if clientsMap != nil {
		client := clientsMap[clientID]
		if client != nil && client.controlConn == conn {
			mediaConn = client.mediaConn
			delete(clientsMap, clientID)
			if len(clientsMap) == 0 {
				delete(rooms, roomID)
			}
			removed = true
		}
	}
	roomsMutex.Unlock()

	if mediaConn != nil {
		_ = mediaConn.Close()
	}

	if removed {
		broadcastRoomInfo(roomID)
	}
}

func removeMediaClient(roomID, clientID string, conn *websocket.Conn) {
	roomsMutex.Lock()
	defer roomsMutex.Unlock()

	clientsMap := rooms[roomID]
	if clientsMap == nil {
		return
	}

	client := clientsMap[clientID]
	if client == nil {
		return
	}

	if client.mediaConn == conn {
		client.mediaConn = nil
	}
}

func handleControlConnections(w http.ResponseWriter, r *http.Request) {
	roomID := getRoomID(r)
	clientID := r.URL.Query().Get("client")
	if clientID == "" {
		http.Error(w, "missing client id", http.StatusBadRequest)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade error: %v", err)
		return
	}

	client, oldControlConn, oldMediaConn, ok := registerControlClient(roomID, clientID, getClientIP(r), ws)
	if !ok {
		_ = ws.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","message":"对讲机模式频道人数已达上限(2人)"}`))
		_ = ws.Close()
		return
	}

	if oldControlConn != nil && oldControlConn != ws {
		_ = oldControlConn.Close()
	}
	if oldMediaConn != nil {
		_ = oldMediaConn.Close()
	}

	broadcastRoomInfo(roomID)

	defer func() {
		removeControlClient(roomID, clientID, ws)
		_ = ws.Close()
	}()

	for {
		_, p, err := ws.ReadMessage()
		if err != nil {
			break
		}

		var msgData struct {
			Type   string `json:"type"`
			Status string `json:"status"`
			ToIP   string `json:"toIP"`
		}
		if err := json.Unmarshal(p, &msgData); err != nil {
			continue
		}

		switch msgData.Type {
		case "update_status":
			if updateClientStatus(roomID, clientID, msgData.Status) {
				broadcastRoomInfo(roomID)
			}
		case "request_talk", "approve_talk":
			forwardControlSignal(roomID, client.id, msgData.Type)
		case "webrtc_offer", "webrtc_answer", "webrtc_candidate":
			forwardWebRTCSignal(roomID, client.id, p)
		case "chat":
			var chatMsgData struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(p, &chatMsgData); err == nil {
				contentRunes := []rune(chatMsgData.Content)
				if len(contentRunes) > 1000 {
					chatMsgData.Content = string(contentRunes[:1000])
				}
				msg := ChatMessage{
					ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
					RoomID:    roomID,
					SenderID:  clientID,
					SenderIP:  client.ip,
					Type:      "text",
					Content:   chatMsgData.Content,
					Timestamp: time.Now().UnixMilli(),
				}
				saveChatMessage(msg)
				broadcastChatMessage(roomID, msg)
			}
		}
	}
}

func handleMediaConnections(w http.ResponseWriter, r *http.Request) {
	roomID := getRoomID(r)
	clientID := r.URL.Query().Get("client")
	if clientID == "" {
		http.Error(w, "missing client id", http.StatusBadRequest)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade error: %v", err)
		return
	}

	client, oldMediaConn, ok := registerMediaClient(roomID, clientID, ws)
	if !ok {
		_ = ws.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","message":"控制通道未建立，无法连接媒体通道"}`))
		_ = ws.Close()
		return
	}

	if oldMediaConn != nil && oldMediaConn != ws {
		_ = oldMediaConn.Close()
	}

	defer func() {
		removeMediaClient(roomID, clientID, ws)
		_ = ws.Close()
	}()

	for {
		messageType, p, err := ws.ReadMessage()
		if err != nil {
			break
		}

		if messageType != websocket.BinaryMessage {
			continue
		}

		forwardMediaMessage(roomID, client.id, messageType, p)
	}
}

func main() {
	// 启动时只加载一次配置，并赋值给全局变量
	appConfig = LoadConfig()

	// 初始化聊天存储
	initChatStorage()

	if err := generateCertIfNotExist(appConfig.CertFile, appConfig.KeyFile); err != nil {
		log.Fatalf("Failed to generate certificate: %v", err)
	}

	// 提取 embed 中的 public 子目录
	publicFS, err := fs.Sub(content, "public")
	if err != nil {
		log.Fatal(err)
	}

	fileServer := http.FileServer(http.FS(publicFS))
	http.Handle("/", fileServer)
	http.HandleFunc("/ws/control", handleControlConnections)
	http.HandleFunc("/ws/media", handleMediaConnections)

	// 聊天相关API
	http.HandleFunc("/api/chat/history", handleChatHistoryAPI)
	http.HandleFunc("/api/chat/upload", handleFileUploadAPI)
	http.HandleFunc("/api/download/", handleFileDownloadAPI)

	// 提供获取音质配置的 API
	http.HandleFunc("/api/audio-config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		// 判断是否有预设
		qualityLabel := appConfig.Quality
		if qualityLabel == "" {
			qualityLabel = "custom"
		}
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mode":       appConfig.Mode,
			"protocol":   appConfig.Protocol,
			"quality":    qualityLabel,
			"sampleRate": appConfig.SampleRate,
			"bufferSize": appConfig.BufferSize,
		})
	})

	// 仅启动 HTTPS 服务
	log.Printf("HTTPS Server starting on %s\n", appConfig.HTTPSPort)
	log.Printf("You can now access https://localhost%s or your local IP address.", appConfig.HTTPSPort)
	if err := http.ListenAndServeTLS(appConfig.HTTPSPort, appConfig.CertFile, appConfig.KeyFile, nil); err != nil {
		log.Fatal("ListenAndServeTLS:", err)
	}
}
