package main

import (
	"flag"
	"os"
	"strconv"
	"strings"
)

// Config 存储应用配置
type Config struct {
	HTTPSPort  string
	CertFile   string
	KeyFile    string
	Mode       string // 模式预设：normal, walkie-talkie
	Protocol   string // 协议预设：ws, webrtc
	Quality    string // 音质预设：lossless, high, medium, low
	SampleRate int
	BufferSize int
}

// LoadConfig 解析命令行参数和环境变量并返回 Config
func LoadConfig() *Config {
	cfg := &Config{}

	httpsPort := getEnv("HTTPS_PORT", ":8443")
	certFile := getEnv("CERT_FILE", "cert.pem")
	keyFile := getEnv("KEY_FILE", "key.pem")
	mode := getEnv("MODE", "normal")
	protocol := getEnv("PROTOCOL", "webrtc")
	quality := getEnv("QUALITY", "lossless")
	sampleRate := getEnvAsInt("SAMPLE_RATE", 48000)
	bufferSize := getEnvAsInt("BUFFER_SIZE", 4096)

	flag.StringVar(&cfg.HTTPSPort, "https-port", httpsPort, "HTTPS server port (alias: -p)")
	flag.StringVar(&cfg.HTTPSPort, "p", httpsPort, "HTTPS server port")
	flag.StringVar(&cfg.CertFile, "cert-file", certFile, "TLS certificate file path (alias: -c)")
	flag.StringVar(&cfg.CertFile, "c", certFile, "TLS certificate file path")
	flag.StringVar(&cfg.KeyFile, "key-file", keyFile, "TLS key file path (alias: -k)")
	flag.StringVar(&cfg.KeyFile, "k", keyFile, "TLS key file path")
	flag.StringVar(&cfg.Mode, "mode", mode, "Operation mode (alias: -m)")
	flag.StringVar(&cfg.Mode, "m", mode, "Operation mode")
	flag.StringVar(&cfg.Protocol, "protocol", protocol, "Media transport protocol (alias: -t)")
	flag.StringVar(&cfg.Protocol, "t", protocol, "Media transport protocol")
	flag.StringVar(&cfg.Quality, "quality", quality, "Audio quality preset (alias: -q)")
	flag.StringVar(&cfg.Quality, "q", quality, "Audio quality preset")
	flag.IntVar(&cfg.SampleRate, "sample-rate", sampleRate, "Audio sample rate in Hz (alias: -s)")
	flag.IntVar(&cfg.SampleRate, "s", sampleRate, "Audio sample rate in Hz")
	flag.IntVar(&cfg.BufferSize, "buffer-size", bufferSize, "Audio buffer size (alias: -b)")
	flag.IntVar(&cfg.BufferSize, "b", bufferSize, "Audio buffer size")
	
	flag.Parse()
	cfg.HTTPSPort = normalizeHTTPSPort(cfg.HTTPSPort)

	// walkie-talkie 模式下强制为无损音质
	if cfg.Mode == "walkie-talkie" {
		cfg.Quality = "lossless"
	}

	// 如果设置了预设配置，则预设优先级高于自定义参数
	switch cfg.Quality {
	case "lossless":
		// 无损模式：将 SampleRate 和 BufferSize 设置为 0，表示由前端浏览器自动决定最佳参数（硬件默认）
		cfg.SampleRate = 0
		cfg.BufferSize = 0
	case "high":
		cfg.SampleRate = 48000
		cfg.BufferSize = 2048
	case "medium":
		cfg.SampleRate = 44100
		cfg.BufferSize = 4096
	case "low":
		cfg.SampleRate = 16000
		cfg.BufferSize = 8192
	}

	return cfg
}

func normalizeHTTPSPort(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ":8443"
	}
	if strings.HasPrefix(value, ":") {
		return value
	}
	if _, err := strconv.Atoi(value); err == nil {
		return ":" + value
	}
	return value
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// getEnvAsInt 获取整数类型的环境变量
func getEnvAsInt(key string, fallback int) int {
	if valueStr, exists := os.LookupEnv(key); exists {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return fallback
}
