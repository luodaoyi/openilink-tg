# Telegram 到微信 iLink 桥接服务设计

## 背景

当前仓库为空仓，需要从零实现一个 Go 服务，对外暴露 Telegram Bot 兼容的 HTTP 接口，
对内调用 `weixin-ilink-sdk-go` 将消息发送到微信 iLink。

## 目标

1. 提供 Telegram Bot 风格的 HTTP 路由。
2. 将 Telegram 的发送请求转换为 iLink SDK 调用。
3. 维护 iLink 所需的 `context_token`，支持服务重启后的状态恢复。
4. 提供 Docker 构建能力。
5. 提供 GitHub Actions 自动测试、构建和发版能力。

## 最小实现范围

首版先支持以下接口：

- `GET/POST /bot<TOKEN>/getMe`
- `POST /bot<TOKEN>/sendMessage`
- `POST /bot<TOKEN>/sendChatAction`
- `GET /healthz`

其中：

- `chat_id` 直接映射为微信 iLink 的用户 ID。
- `sendMessage` 仅支持文本消息。
- `sendChatAction` 将 Telegram 的动作统一映射为微信侧 `typing` 状态。

## 架构设计

### 1. 配置层

通过环境变量加载运行配置：

- HTTP 监听地址
- Telegram 兼容 token
- iLink token / base URL
- 状态持久化文件路径
- 服务展示信息（bot name / username）

### 2. 适配层

服务内部定义桥接接口，屏蔽 Telegram 与微信 SDK 的差异：

- `GetMe`
- `SendText`
- `SendTyping`

这样 HTTP 层可以用假实现测试，不依赖真实微信网络调用。

### 3. 状态层

iLink 主动发消息依赖历史消息中的 `context_token`。
因此服务启动后会：

- 从本地状态文件恢复 `sync_buf` 和 `user_id -> context_token`
- 启动后台 monitor 循环
- 持续刷新本地状态文件

### 4. HTTP 兼容层

按 Telegram Bot API 风格输出：

- 成功：`{ "ok": true, "result": ... }`
- 失败：`{ "ok": false, "error_code": N, "description": "..." }`

## 错误策略

- token 不匹配：`401 Unauthorized`
- 参数缺失：`400 Bad Request`
- 找不到 `context_token`：`400 Bad Request`
- 微信侧调用失败：`502 Bad Gateway`

## 测试策略

优先覆盖：

1. Telegram 路由鉴权
2. `sendMessage` 的 JSON / 表单兼容
3. Telegram 错误响应格式
4. 状态文件读写

