# openilink-tg

一个将 Telegram Bot 兼容 HTTP 接口转换为微信 iLink SDK 调用的 Go 服务。

## 当前能力

当前已支持：

- `GET /bot<TOKEN>/getMe`
- `POST /bot<TOKEN>/sendMessage`
- `POST /bot<TOKEN>/sendPhoto`
- `POST /bot<TOKEN>/sendDocument`
- `POST /bot<TOKEN>/sendVideo`
- `POST /bot<TOKEN>/sendVoice`
- `POST /bot<TOKEN>/sendAudio`
- `POST /bot<TOKEN>/sendAnimation`
- `POST /bot<TOKEN>/sendChatAction`
- `GET /healthz`

约束说明：

- `chat_id` 直接映射为微信 iLink 的用户 ID。
- `sendMessage` 支持文本消息。
- `sendPhoto` / `sendDocument` / `sendVideo` / `sendVoice` / `sendAudio` / `sendAnimation` 当前支持 `multipart/form-data` 文件上传，并透传 `caption`。
- `sendChatAction` 当前统一映射为微信侧 `typing`。
- 主动推送依赖历史入站消息中的 `context_token`。

## 运行原理

服务内部做两件事：

1. 对外暴露 Telegram Bot 风格 HTTP 路由。
2. 对内启动 iLink monitor 长轮询，持续维护：
   - `get_updates_buf`
   - `user_id -> context_token`

服务会将这两类状态持久化到本地文件，便于重启恢复。

## 环境变量

| 变量名 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `TELEGRAM_BOT_TOKEN` | 是 | - | Telegram 兼容层访问 token |
| `ILINK_TOKEN` | 是 | - | 微信 iLink bot token |
| `ILINK_BASE_URL` | 否 | SDK 默认值 | 微信 iLink API 基础地址 |
| `HTTP_ADDR` | 否 | `:8080` | HTTP 监听地址 |
| `STATE_FILE` | 否 | `data/state.json` | 上下文状态文件 |
| `BOT_ID` | 否 | `1` | `getMe` 返回的 bot id |
| `BOT_NAME` | 否 | `Weixin iLink Bridge` | `getMe` 返回的 bot 名称 |
| `BOT_USERNAME` | 否 | `openilink_tg_bot` | `getMe` 返回的用户名 |
| `MONITOR_ENABLED` | 否 | `true` | 是否启动 iLink monitor |

## 本地运行

```bash
go run ./cmd/openilink-tg
```

## 编译

```bash
go build ./...
```

## 测试

```bash
go test ./...
```

## 示例请求

### `getMe`

```bash
curl "http://localhost:8080/bot${TELEGRAM_BOT_TOKEN}/getMe"
```

### `sendMessage`

```bash
curl -X POST "http://localhost:8080/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": "wx-user-id",
    "text": "hello from telegram bridge"
  }'
```

### `sendPhoto`

```bash
curl -X POST "http://localhost:8080/bot${TELEGRAM_BOT_TOKEN}/sendPhoto" \
  -F chat_id=wx-user-id \
  -F caption='hello image' \
  -F photo=@/path/to/photo.jpg
```

### `sendDocument`

```bash
curl -X POST "http://localhost:8080/bot${TELEGRAM_BOT_TOKEN}/sendDocument" \
  -F chat_id=wx-user-id \
  -F caption='hello file' \
  -F document=@/path/to/file.pdf
```

### `sendVideo`

```bash
curl -X POST "http://localhost:8080/bot${TELEGRAM_BOT_TOKEN}/sendVideo" \
  -F chat_id=wx-user-id \
  -F caption='hello video' \
  -F video=@/path/to/video.mp4
```

### `sendVoice`

```bash
curl -X POST "http://localhost:8080/bot${TELEGRAM_BOT_TOKEN}/sendVoice" \
  -F chat_id=wx-user-id \
  -F caption='hello voice' \
  -F voice=@/path/to/voice.ogg
```

### `sendAudio`

```bash
curl -X POST "http://localhost:8080/bot${TELEGRAM_BOT_TOKEN}/sendAudio" \
  -F chat_id=wx-user-id \
  -F caption='hello audio' \
  -F audio=@/path/to/audio.mp3
```

### `sendAnimation`

```bash
curl -X POST "http://localhost:8080/bot${TELEGRAM_BOT_TOKEN}/sendAnimation" \
  -F chat_id=wx-user-id \
  -F caption='hello gif' \
  -F animation=@/path/to/animation.gif
```

### `sendChatAction`

```bash
curl -X POST "http://localhost:8080/bot${TELEGRAM_BOT_TOKEN}/sendChatAction" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": "wx-user-id",
    "action": "typing"
  }'
```

## Docker

项目包含多阶段 `Dockerfile`，可直接构建：

```bash
docker build -t openilink-tg .
```

## 发版策略

GitHub Actions 负责两类自动化：

1. `ci.yml`
   - 在 `push` / `pull_request` 时运行 `go test` 与 `go build`
2. `release.yml`
   - 在 tag 推送时构建多平台二进制
   - 自动创建 GitHub Release

## 后续可扩展方向

- URL 型媒体透传（而不只是 multipart 上传）
- QR 登录辅助接口
- 更丰富的 Telegram Bot API 兼容面

## 许可证

本项目使用 MIT License，详见 `LICENSE`。
