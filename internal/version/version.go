package version

var (
	// Version 由构建阶段通过 ldflags 注入。
	Version = "dev"
	// Commit 由构建阶段通过 ldflags 注入。
	Commit = "unknown"
	// BuildDate 由构建阶段通过 ldflags 注入。
	BuildDate = "unknown"
)
