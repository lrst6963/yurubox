package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
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
	id              string
	ip              string
	name            string
	avatar          string
	status          string
	hasVideo        bool
	mutedUntil      int64
	mediaMutedUntil int64
	controlConn     *websocket.Conn
	mediaConn       *websocket.Conn
	writeMu         sync.Mutex
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
	ID         string `json:"id"`
	IP         string `json:"ip"`
	Name       string `json:"name"`
	Avatar     string `json:"avatar"`
	Status     string `json:"status"`
	Video      bool   `json:"video"`
	IsAdmin    bool   `json:"isAdmin"`
	MediaMuted bool   `json:"mediaMuted"`
	TextMuted  bool   `json:"textMuted"`
}

func isAdminIP(ipStr string) bool {
	if ipStr == "localhost" || ipStr == "::1" || ipStr == "127.0.0.1" {
		return true
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
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
			ID:         c.id,
			IP:         c.ip,
			Name:       c.name,
			Avatar:     c.avatar,
			Status:     c.status,
			Video:      c.hasVideo,
			IsAdmin:    isAdminIP(c.ip),
			MediaMuted: c.mediaMutedUntil > time.Now().UnixMilli(),
			TextMuted:  c.mutedUntil > time.Now().UnixMilli(),
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

func normalizeClientName(name string) string {
	name = strings.TrimSpace(name)
	nameRunes := []rune(name)
	if len(nameRunes) > 20 {
		name = string(nameRunes[:20])
	}
	if name == "" {
		return "未命名"
	}
	return name
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

func registerControlClient(roomID, clientID, ip, name, avatar string, conn *websocket.Conn) (*Client, *websocket.Conn, *websocket.Conn, bool) {
	var oldControlConn *websocket.Conn
	var oldMediaConn *websocket.Conn
	normalizedName := normalizeClientName(name)

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
		client = &Client{id: clientID, ip: ip, name: normalizedName, avatar: avatar, status: "就绪", hasVideo: false}
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
	client.name = normalizedName
	client.avatar = avatar
	client.status = "就绪"
	client.hasVideo = false
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

func updateClientVideoState(roomID, clientID string, hasVideo bool) bool {
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

	client.hasVideo = hasVideo
	return true
}

func updateClientName(roomID, clientID, name string) bool {
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

	client.name = normalizeClientName(name)
	return true
}

func updateClientAvatar(roomID, clientID, avatar string) bool {
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

	client.avatar = avatar
	return true
}

func isClientMuted(roomID, clientID string) (bool, int64) {
	roomsMutex.Lock()
	defer roomsMutex.Unlock()
	if rooms[roomID] != nil && rooms[roomID][clientID] != nil {
		mutedUntil := rooms[roomID][clientID].mutedUntil
		if time.Now().UnixMilli() < mutedUntil {
			return true, mutedUntil
		}
	}
	return false, 0
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

	client, oldControlConn, oldMediaConn, ok := registerControlClient(roomID, clientID, getClientIP(r), r.URL.Query().Get("name"), r.URL.Query().Get("avatar"), ws)
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
			Video  bool   `json:"video"`
		}
		if err := json.Unmarshal(p, &msgData); err != nil {
			continue
		}

		switch msgData.Type {
		case "update_status":
			if updateClientStatus(roomID, clientID, msgData.Status) {
				broadcastRoomInfo(roomID)
			}
		case "update_video":
			if updateClientVideoState(roomID, clientID, msgData.Video) {
				broadcastRoomInfo(roomID)
			}
		case "update_name":
			var nameMsgData struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(p, &nameMsgData); err == nil && updateClientName(roomID, clientID, nameMsgData.Name) {
				broadcastRoomInfo(roomID)
			}
		case "update_avatar":
			var avatarMsgData struct {
				Avatar string `json:"avatar"`
			}
			if err := json.Unmarshal(p, &avatarMsgData); err == nil && updateClientAvatar(roomID, clientID, avatarMsgData.Avatar) {
				broadcastRoomInfo(roomID)
			}
		case "request_talk", "approve_talk":
			forwardControlSignal(roomID, client.id, msgData.Type)
		case "webrtc_offer", "webrtc_answer", "webrtc_candidate":
			forwardWebRTCSignal(roomID, client.id, p)
		case "chat":
			if client.mutedUntil > time.Now().UnixMilli() {
				_ = writeMessage(client, ws, websocket.TextMessage, []byte(fmt.Sprintf(`{"type":"error","message":"您已被禁言至 %s"}`, time.UnixMilli(client.mutedUntil).Format("15:04:05"))))
				continue
			}
			var chatMsgData struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(p, &chatMsgData); err == nil {
				contentRunes := []rune(chatMsgData.Content)
				if len(contentRunes) > 1000 {
					chatMsgData.Content = string(contentRunes[:1000])
				}
				msg := ChatMessage{
					ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
					RoomID:     roomID,
					SenderID:   clientID,
					SenderIP:   client.ip,
					SenderName: client.name,
					SenderAvatar: client.avatar,
					Type:       "text",
					Content:    chatMsgData.Content,
					Timestamp:  time.Now().UnixMilli(),
				}
				saveChatMessage(msg)
				broadcastChatMessage(roomID, msg)
			}
		case "admin_action":
			if !isAdminIP(client.ip) {
				_ = writeMessage(client, ws, websocket.TextMessage, []byte(`{"type":"error","message":"无权限"}`))
				continue
			}
			var adminData struct {
				Action   string `json:"action"`
				TargetID string `json:"targetID"`
				Duration int64  `json:"duration"` // 禁言时长(毫秒)
				NewName  string `json:"newName"`
			}
			if err := json.Unmarshal(p, &adminData); err != nil {
				continue
			}

			roomsMutex.Lock()
			targetClient := rooms[roomID][adminData.TargetID]
			roomsMutex.Unlock()

			if targetClient == nil {
				continue
			}

			if adminData.Action == "kick" {
				if targetClient.controlConn != nil {
					_ = writeMessage(targetClient, targetClient.controlConn, websocket.TextMessage, []byte(`{"type":"kicked","message":"您已被管理员踢出频道"}`))
					targetClient.controlConn.Close()
				}
			} else if adminData.Action == "mute" {
				roomsMutex.Lock()
				targetClient.mutedUntil = time.Now().UnixMilli() + adminData.Duration
				roomsMutex.Unlock()
				if targetClient.controlConn != nil {
					_ = writeMessage(targetClient, targetClient.controlConn, websocket.TextMessage, []byte(fmt.Sprintf(`{"type":"muted","message":"您已被管理员禁言"}`)))
				}
				broadcastRoomInfo(roomID)
				// 广播系统消息
				durationSec := adminData.Duration / 1000
				sysMsg := ChatMessage{
					ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
					RoomID:     roomID,
					Type:       "system",
					Content:    fmt.Sprintf("%s 被管理员禁言 %d 秒", targetClient.name, durationSec),
					Timestamp:  time.Now().UnixMilli(),
				}
				saveChatMessage(sysMsg)
				broadcastChatMessage(roomID, sysMsg)
			} else if adminData.Action == "mute_media" {
				roomsMutex.Lock()
				targetClient.mediaMutedUntil = time.Now().UnixMilli() + adminData.Duration
				roomsMutex.Unlock()
				if targetClient.controlConn != nil {
					_ = writeMessage(targetClient, targetClient.controlConn, websocket.TextMessage, []byte(fmt.Sprintf(`{"type":"media_muted","message":"您的语音/视频已被管理员禁用"}`)))
				}
				broadcastRoomInfo(roomID)
				
				// 广播系统消息
				durationSec := adminData.Duration / 1000
				sysMsg := ChatMessage{
					ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
					RoomID:     roomID,
					Type:       "system",
					Content:    fmt.Sprintf("%s 被管理员禁音(语音/视频) %d 秒", targetClient.name, durationSec),
					Timestamp:  time.Now().UnixMilli(),
				}
				saveChatMessage(sysMsg)
				broadcastChatMessage(roomID, sysMsg)
			} else if adminData.Action == "unmute_all" {
				roomsMutex.Lock()
				targetClient.mutedUntil = 0
				targetClient.mediaMutedUntil = 0
				roomsMutex.Unlock()
				if targetClient.controlConn != nil {
					_ = writeMessage(targetClient, targetClient.controlConn, websocket.TextMessage, []byte(`{"type":"unmuted","message":"您的禁言禁音已被解除"}`))
				}
				broadcastRoomInfo(roomID)

				// 广播系统消息
				sysMsg := ChatMessage{
					ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
					RoomID:     roomID,
					Type:       "system",
					Content:    fmt.Sprintf("%s 的禁言/禁音已被解除", targetClient.name),
					Timestamp:  time.Now().UnixMilli(),
				}
				saveChatMessage(sysMsg)
				broadcastChatMessage(roomID, sysMsg)
			} else if adminData.Action == "change_name" {
				if updateClientName(roomID, adminData.TargetID, adminData.NewName) {
					broadcastRoomInfo(roomID)
				}
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
	http.HandleFunc("/api/chat/upload-images", handleBatchImageUploadAPI)
	http.HandleFunc("/api/chat/revoke", handleChatRevokeAPI)
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
