// Package web 提供嵌入 Go 服务的前端静态资源。
package web

import "embed"

// FS 是前端静态资源文件系统。
//
//go:embed static
var FS embed.FS
