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
	SenderID  string `json:"senderId"`
	SenderIP  string `json:"senderIp"`
	Type      string `json:"type"`    // "text", "image", "file"
	Content   string `json:"content"` // Text content or file URL path
	FileName  string `json:"fileName,omitempty"`
	FileSize  int64  `json:"fileSize,omitempty"`
	Timestamp int64  `json:"timestamp"`
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

	// 生成安全的文件名
	ext := filepath.Ext(header.Filename)
	filenameHash := sha256.Sum256([]byte(fmt.Sprintf("%s_%d", header.Filename, time.Now().UnixNano())))
	safeFilename := hex.EncodeToString(filenameHash[:]) + ext
	savePath := filepath.Join("Data", "uploads", safeFilename)

	dst, err := os.Create(savePath)
	if err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	roomsMutex.Lock()
	senderIP := ""
	if cmap, ok := rooms[roomID]; ok {
		if c, ok := cmap[senderID]; ok {
			senderIP = c.ip
		}
	}
	roomsMutex.Unlock()

	// 创建聊天消息记录
	msg := ChatMessage{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		RoomID:    roomID,
		SenderID:  senderID,
		SenderIP:  senderIP,
		Type:      fileType,
		Content:   "/api/download/" + safeFilename,
		FileName:  header.Filename,
		FileSize:  header.Size,
		Timestamp: time.Now().UnixMilli(),
	}

	if err := saveChatMessage(msg); err != nil {
		log.Printf("Failed to save chat message: %v", err)
	}

	// 广播消息
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

	http.ServeFile(w, r, filePath)
}

// broadcastChatMessage 广播聊天消息给频道内的所有人
func broadcastChatMessage(roomID string, msg ChatMessage) {
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
		Type: "chat_message",
		Data: msg,
	})

	for _, c := range recipients {
		_ = writeMessage(c, c.controlConn, websocket.TextMessage, payload)
	}
}
