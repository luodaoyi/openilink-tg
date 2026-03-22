# Telegram Bridge Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 构建一个将 Telegram Bot 兼容接口转换为微信 iLink SDK 调用的 Go 服务。

**Architecture:** 使用标准库 `net/http` 暴露 Telegram 风格路由，使用内部桥接接口隔离协议转换，后台使用 iLink SDK monitor 持续维护上下文 token 与同步游标。

**Tech Stack:** Go、标准库 `net/http`、`weixin-ilink-sdk-go`、Docker、GitHub Actions

---

### Task 1: 建立项目骨架与失败测试

**Files:**
- Create: `go.mod`
- Create: `internal/httpapi/handler_test.go`
- Create: `internal/state/file_store_test.go`

**Step 1: Write the failing test**

编写以下失败测试：

- 有效 token 访问 `getMe` 返回 Telegram 风格成功响应
- `sendMessage` JSON 请求可被正确转换
- 缺少上下文 token 时返回 Telegram 风格错误
- 状态文件可持久化并恢复

**Step 2: Run test to verify it fails**

Run: `go test ./...`
Expected: FAIL，因为实现尚不存在。

**Step 3: Write minimal implementation**

补齐最小 HTTP 处理器、桥接接口和状态存储。

**Step 4: Run test to verify it passes**

Run: `go test ./...`
Expected: PASS

### Task 2: 实现运行时服务与微信适配

**Files:**
- Create: `cmd/openilink-tg/main.go`
- Create: `internal/app/app.go`
- Create: `internal/config/config.go`
- Create: `internal/bridge/service.go`
- Create: `internal/version/version.go`

**Step 1: Write the failing test**

补充配置解析和桥接错误映射测试。

**Step 2: Run test to verify it fails**

Run: `go test ./...`
Expected: FAIL

**Step 3: Write minimal implementation**

接入 `weixin-ilink-sdk-go`，初始化 monitor 与状态恢复，启动 HTTP 服务。

**Step 4: Run test to verify it passes**

Run: `go test ./...`
Expected: PASS

### Task 3: 完成容器化与自动发布

**Files:**
- Create: `Dockerfile`
- Create: `.dockerignore`
- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/release.yml`
- Create: `README.md`

**Step 1: Write the failing test**

这部分以构建验证代替单元测试：

- `docker build .`
- `go test ./...`

**Step 2: Run command to verify it fails if misconfigured**

Expected: 若 Dockerfile 或 workflow 缺失则无法满足交付要求。

**Step 3: Write minimal implementation**

添加多阶段构建、CI 与 release workflow，并补充运行说明。

**Step 4: Run verification**

Run:

- `go test ./...`
- `go build ./...`

Expected: PASS
