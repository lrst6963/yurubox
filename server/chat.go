package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ChatMessage struct {
	ID        string `json:"id"`
	RoomID    string `json:"roomId"`
	SenderID   string `json:"senderId"`
	SenderIP   string `json:"senderIp"`
	SenderName string `json:"senderName,omitempty"`
	SenderAvatar string `json:"senderAvatar,omitempty"`
	Type       string `json:"type"`    // "text", "image", "file"
	Content   string `json:"content"` // Text content or file URL path
	FileName  string `json:"fileName,omitempty"`
	FileSize  int64  `json:"fileSize,omitempty"`
	Images    []ChatImage `json:"images,omitempty"`
	Timestamp int64  `json:"timestamp"`
	Revoked   bool   `json:"revoked,omitempty"`
	RevokedAt int64  `json:"revokedAt,omitempty"`
}

type ChatImage struct {
	URL      string `json:"url"`
	FileName string `json:"fileName,omitempty"`
	FileSize int64  `json:"fileSize,omitempty"`
}

var (
	chatMutex sync.Mutex
)

// initChatStorage 初始化聊天存储，启动时清空 Data 目录
func initChatStorage() {
	err := os.RemoveAll("Data")
	if err != nil {
		log.Printf("Failed to remove Data dir: %v", err)
	}
	err = os.MkdirAll("Data/uploads", 0755)
	if err != nil {
		log.Fatalf("Failed to create Data dir: %v", err)
	}
}

// hashRoomID 将频道ID哈希化以用作文件名
func hashRoomID(roomID string) string {
	hash := sha256.Sum256([]byte(roomID))
	return hex.EncodeToString(hash[:])
}

// saveChatMessage 保存聊天消息到频道的JSON文件中
func saveChatMessage(msg ChatMessage) error {
	chatMutex.Lock()
	defer chatMutex.Unlock()

	roomHash := hashRoomID(msg.RoomID)
	filePath := filepath.Join("Data", roomHash+".json")

	var messages []ChatMessage

	// 读取现有消息
	data, err := os.ReadFile(filePath)
	if err == nil {
		json.Unmarshal(data, &messages)
	}

	messages = append(messages, msg)

	// 写入文件
	newData, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, newData, 0644)
}

// getChatHistory 获取频道的聊天历史
func getChatHistory(roomID string) []ChatMessage {
	chatMutex.Lock()
	defer chatMutex.Unlock()

	roomHash := hashRoomID(roomID)
	filePath := filepath.Join("Data", roomHash+".json")

	var messages []ChatMessage
	data, err := os.ReadFile(filePath)
	if err == nil {
		json.Unmarshal(data, &messages)
	}

	return messages
}

func getSenderInfo(roomID, senderID string) (ip string, name string, avatar string) {
	roomsMutex.Lock()
	defer roomsMutex.Unlock()

	if cmap, ok := rooms[roomID]; ok {
		if c, ok := cmap[senderID]; ok {
			return c.ip, c.name, c.avatar
		}
	}
	return "", "", ""
}

func saveUploadedFile(file io.Reader, filename string) (string, error) {
	ext := filepath.Ext(filename)
	filenameHash := sha256.Sum256([]byte(fmt.Sprintf("%s_%d", filename, time.Now().UnixNano())))
	safeFilename := hex.EncodeToString(filenameHash[:]) + ext
	savePath := filepath.Join("Data", "uploads", safeFilename)

	dst, err := os.Create(savePath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}

	return safeFilename, nil
}

func getMessageUploadFilenames(msg ChatMessage) []string {
	filenames := make([]string, 0, 1+len(msg.Images))
	if msg.Type == "file" || msg.Type == "image" {
		if msg.Content != "" {
			filename := filepath.Base(msg.Content)
			if filename != "." && filename != "/" && filename != "" {
				filenames = append(filenames, filename)
			}
		}
	}
	for _, image := range msg.Images {
		filename := filepath.Base(image.URL)
		if filename != "." && filename != "/" && filename != "" {
			filenames = append(filenames, filename)
		}
	}
	return filenames
}

func deleteUploadedFiles(filenames []string) {
	for _, filename := range filenames {
		if filename == "" {
			continue
		}
		filePath := filepath.Join("Data", "uploads", filepath.Base(filename))
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			log.Printf("Failed to delete uploaded file %s: %v", filename, err)
		}
	}
}

func isFileReferenced(filename string) bool {
	chatMutex.Lock()
	defer chatMutex.Unlock()

	entries, err := os.ReadDir("Data")
	if err != nil {
		return false
	}

	targetFilename := filepath.Base(filename)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join("Data", entry.Name())
		data, readErr := os.ReadFile(filePath)
		if readErr != nil {
			continue
		}

		var messages []ChatMessage
		if err := json.Unmarshal(data, &messages); err != nil {
			continue
		}

		for _, msg := range messages {
			if msg.Revoked {
				continue
			}
			for _, referencedFile := range getMessageUploadFilenames(msg) {
				if referencedFile == targetFilename {
					return true
				}
			}
		}
	}

	return false
}

func revokeChatMessage(roomID, clientID, clientIP, messageID string) (ChatMessage, error) {
	chatMutex.Lock()
	defer chatMutex.Unlock()

	roomHash := hashRoomID(roomID)
	filePath := filepath.Join("Data", roomHash+".json")

	var messages []ChatMessage
	data, err := os.ReadFile(filePath)
	if err == nil {
		_ = json.Unmarshal(data, &messages)
	}

	for i := range messages {
		if messages[i].ID != messageID {
			continue
		}
		
		if messages[i].SenderID != clientID {
			isRevokerSuper := isAdminIP(clientIP)
			var isRevokerAdmin bool
			roomsMutex.Lock()
			if cmap, ok := rooms[roomID]; ok {
				if c, ok := cmap[clientID]; ok {
					isRevokerAdmin = c.isAdmin || isRevokerSuper
				}
			}
			roomsMutex.Unlock()

			if !isRevokerAdmin {
				return ChatMessage{}, fmt.Errorf("只能撤回自己的消息")
			}

			if isAdminIP(messages[i].SenderIP) && !isRevokerSuper {
				return ChatMessage{}, fmt.Errorf("无权撤回超级管理员的消息")
			}
		} else {
			isRevokerSuper := isAdminIP(clientIP)
			var isRevokerAdmin bool
			roomsMutex.Lock()
			if cmap, ok := rooms[roomID]; ok {
				if c, ok := cmap[clientID]; ok {
					isRevokerAdmin = c.isAdmin || isRevokerSuper
				}
			}
			roomsMutex.Unlock()

			if time.Since(time.UnixMilli(messages[i].Timestamp)) > 2*time.Minute && !isRevokerAdmin {
				return ChatMessage{}, fmt.Errorf("消息已超过两分钟，无法撤回")
			}
		}

		if messages[i].Revoked {
			return ChatMessage{}, fmt.Errorf("消息已经撤回")
		}

		filesToDelete := getMessageUploadFilenames(messages[i])
		messages[i].Revoked = true
		messages[i].RevokedAt = time.Now().UnixMilli()
		messages[i].Content = ""
		messages[i].FileName = ""
		messages[i].FileSize = 0
		messages[i].Images = nil

		newData, marshalErr := json.MarshalIndent(messages, "", "  ")
		if marshalErr != nil {
			return ChatMessage{}, marshalErr
		}
		if writeErr := os.WriteFile(filePath, newData, 0644); writeErr != nil {
			return ChatMessage{}, writeErr
		}
		deleteUploadedFiles(filesToDelete)
		return messages[i], nil
	}

	return ChatMessage{}, fmt.Errorf("消息不存在")
}

// handleChatHistoryAPI 处理获取聊天历史的API请求
func handleChatHistoryAPI(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		roomID = "default"
	}

	messages := getChatHistory(roomID)
	if messages == nil {
		messages = []ChatMessage{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

func handleChatRevokeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		RoomID    string `json:"roomId"`
		ClientID  string `json:"clientId"`
		MessageID string `json:"messageId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if request.RoomID == "" || request.ClientID == "" || request.MessageID == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	clientIP := getClientIP(r)
	message, err := revokeChatMessage(request.RoomID, request.ClientID, clientIP, request.MessageID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	broadcastChatEvent(request.RoomID, "chat_message_revoked", message)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(message)
}

// handleFileUploadAPI 处理文件上传（图片和附件）
func handleFileUploadAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	roomID := r.FormValue("room")
	if roomID == "" {
		roomID = "default"
	}
	senderID := r.FormValue("client")
	if senderID == "" {
		http.Error(w, "Missing client ID", http.StatusBadRequest)
		return
	}

	fileType := r.FormValue("type") // "image" or "file"
	if fileType != "image" && fileType != "file" {
		http.Error(w, "Invalid file type", http.StatusBadRequest)
		return
	}

	// 限制文件大小
	var maxMemory int64
	if fileType == "image" {
		maxMemory = 20 << 20 // 20 MB
	} else {
		maxMemory = 100 << 20 // 100 MB
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		http.Error(w, "File too large or invalid", http.StatusRequestEntityTooLarge)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	safeFilename, err := saveUploadedFile(file, header.Filename)
	if err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}
	senderIP, senderName, senderAvatar := getSenderInfo(roomID, senderID)

	if muted, mutedUntil := isClientMuted(roomID, senderID); muted {
		http.Error(w, fmt.Sprintf("您已被禁言至 %s", time.UnixMilli(mutedUntil).Format("15:04:05")), http.StatusForbidden)
		return
	}

	// 创建聊天消息记录
	msg := ChatMessage{
		ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
		RoomID:     roomID,
		SenderID:   senderID,
		SenderIP:   senderIP,
		SenderName: senderName,
		SenderAvatar: senderAvatar,
		Type:       fileType,
		Content:    "/api/download/" + safeFilename,
		FileName:   header.Filename,
		FileSize:   header.Size,
		Timestamp:  time.Now().UnixMilli(),
	}

	if err := saveChatMessage(msg); err != nil {
		log.Printf("Failed to save chat message: %v", err)
	}

	// 广播消息
	broadcastChatMessage(roomID, msg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

func handleBatchImageUploadAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	roomID := r.FormValue("room")
	if roomID == "" {
		roomID = "default"
	}
	senderID := r.FormValue("client")
	if senderID == "" {
		http.Error(w, "Missing client ID", http.StatusBadRequest)
		return
	}

	const maxImageCount = 9
	const maxImageSize = 20 << 20
	const maxBatchMemory = maxImageCount * maxImageSize

	r.Body = http.MaxBytesReader(w, r.Body, maxBatchMemory)
	if err := r.ParseMultipartForm(maxBatchMemory); err != nil {
		http.Error(w, "Files too large or invalid", http.StatusRequestEntityTooLarge)
		return
	}

	headers := r.MultipartForm.File["files"]
	if len(headers) == 0 {
		http.Error(w, "No images uploaded", http.StatusBadRequest)
		return
	}
	if len(headers) > maxImageCount {
		http.Error(w, "最多一次发送 9 张图片", http.StatusBadRequest)
		return
	}

	images := make([]ChatImage, 0, len(headers))
	for _, header := range headers {
		if header.Size > maxImageSize {
			http.Error(w, "单张图片限制 20MB", http.StatusBadRequest)
			return
		}

		src, err := header.Open()
		if err != nil {
			http.Error(w, "Error retrieving image", http.StatusBadRequest)
			return
		}

		safeFilename, saveErr := saveUploadedFile(src, header.Filename)
		_ = src.Close()
		if saveErr != nil {
			http.Error(w, "Error saving image", http.StatusInternalServerError)
			return
		}

		images = append(images, ChatImage{
			URL:      "/api/download/" + safeFilename,
			FileName: header.Filename,
			FileSize: header.Size,
		})
	}

	senderIP, senderName, senderAvatar := getSenderInfo(roomID, senderID)

	if muted, mutedUntil := isClientMuted(roomID, senderID); muted {
		http.Error(w, fmt.Sprintf("您已被禁言至 %s", time.UnixMilli(mutedUntil).Format("15:04:05")), http.StatusForbidden)
		return
	}

	msg := ChatMessage{
		ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
		RoomID:     roomID,
		SenderID:   senderID,
		SenderIP:   senderIP,
		SenderName: senderName,
		SenderAvatar: senderAvatar,
		Type:       "image_group",
		Images:     images,
		Timestamp:  time.Now().UnixMilli(),
	}

	if err := saveChatMessage(msg); err != nil {
		log.Printf("Failed to save image group message: %v", err)
	}

	broadcastChatMessage(roomID, msg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

// handleFileDownloadAPI 处理文件下载
func handleFileDownloadAPI(w http.ResponseWriter, r *http.Request) {
	filename := filepath.Base(r.URL.Path)
	filePath := filepath.Join("Data", "uploads", filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	if !isFileReferenced(filename) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, filePath)
}

// broadcastChatMessage 广播聊天消息给频道内的所有人
func broadcastChatEvent(roomID, eventType string, msg ChatMessage) {
	roomsMutex.Lock()
	clientsMap, ok := rooms[roomID]
	if !ok {
		roomsMutex.Unlock()
		return
	}

	recipients := make([]*Client, 0, len(clientsMap))
	for _, c := range clientsMap {
		if c.controlConn != nil {
			recipients = append(recipients, c)
		}
	}
	roomsMutex.Unlock()

	type wsMessage struct {
		Type string      `json:"type"`
		Data ChatMessage `json:"data"`
	}

	payload, _ := json.Marshal(wsMessage{
		Type: eventType,
		Data: msg,
	})

	for _, c := range recipients {
		_ = writeMessage(c, c.controlConn, websocket.TextMessage, payload)
	}
}

func broadcastChatMessage(roomID string, msg ChatMessage) {
	broadcastChatEvent(roomID, "chat_message", msg)
}
