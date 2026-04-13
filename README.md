# phonecall

一个简约的Web实时聊天与音视频通话室。

<img width="2560" height="1287" alt="image" src="https://github.com/user-attachments/assets/7a1e672b-1ff2-4c23-b3a9-812b38ae2d7c" />


## ✨ 功能特性

- **多频道/房间支持**：支持通过密码创建和加入独立的加密频道。
- **音视频通话**：基于 WebRTC 的点对点实时音视频通信。
- **实时消息**：基于 WebSocket 的实时文本聊天，支持历史消息查询和消息撤回。
- **文件与图片分享**：支持在聊天中上传并发送图片和文件。
- **灵活的音质控制**：支持无损、高、中、低多种预设音质，也可自定义采样率和缓冲区大小。
- **现代化 UI**：采用 Vue 3 + Material Design Web 构建，响应式设计。
- **自动 HTTPS**：内建自签名证书生成，默认支持安全连接（HTTPS/TLS）。

## 🛠 技术栈

- **后端**：Go 1.16+ (Gorilla WebSocket)
- **前端**：Vue 3, Vite, TypeScript, Material Web, WebRTC

## 📦 编译与运行

### 前置要求

- Go 1.16 或更高版本
- Node.js 16 或更高版本

### 编译步骤

你可以使用提供的 `Makefile` 方便地进行编译：

```bash
# 编译完整项目（包含前端）
make build

# 仅编译前端
make web

# 清理构建产物
make clean
```

编译完成后会生成 `phonecall` 可执行文件。

## 🚀 使用

### 运行应用

```bash
# 直接运行（默认端口 8443）
./phonecall

# 或者通过环境变量配置运行
HTTPS_PORT=:8443 MODE=normal ./phonecall
```

在浏览器中打开应用地址（默认为 `https://{DOMAIN}:8443` 或 `https://localhost:8443`）即可使用。
*注：由于使用了自签名证书，首次访问时浏览器可能会提示不安全，请选择继续访问。*

## ⚙️ 配置参数

应用支持通过环境变量或命令行参数进行配置：

| 命令行参数 | 环境变量 | 默认值 | 说明 |
|------|----------|--------|------|
| `-p` | `HTTPS_PORT` | `:8443` | HTTPS 服务监听端口 |
| `-c` | `CERT_FILE` | `cert.pem` | TLS 证书文件路径（如果不存在会自动生成） |
| `-k` | `KEY_FILE` | `key.pem` | TLS 私钥文件路径 |
| `-m` | `MODE` | `normal` | 运行模式：`normal` 或 `walkie-talkie` |
| `-t` | `PROTOCOL` | `webrtc` | 媒体传输协议：`webrtc` 或 `ws` |
| `-q` | `QUALITY` | `lossless`| 音质预设：`lossless`, `high`, `medium`, `low` |
| `-s` | `SAMPLE_RATE`| `48000` | 音频采样率 (Hz) |
| `-b` | `BUFFER_SIZE`| `4096` | 音频缓冲区大小 |
