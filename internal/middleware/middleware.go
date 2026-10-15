// Package middleware 提供 HTTP 中间件：请求日志、异常恢复、安全头。
package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/smash-hq/server-management/internal/logger"
)

// Logging 记录请求日志：错误和慢请求记录 INFO，其余记录 DEBUG。
func Logging(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(ww, r)

			duration := time.Since(start)
			// 静态资源、健康检查、WebSocket 使用 DEBUG 级别
			isStatic := strings.HasPrefix(r.URL.Path, "/css/") ||
				strings.HasPrefix(r.URL.Path, "/js/") ||
				strings.HasPrefix(r.URL.Path, "/favicon") ||
				r.URL.Path == "/api/v1/health" ||
				r.URL.Path == "/ws"
			isError := ww.status >= 400
			isSlow := duration > 2*time.Second

			if isError || isSlow {
				log.Warnw("request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", ww.status,
					"duration", duration.String(),
					"remote", r.RemoteAddr,
				)
			} else if !isStatic {
				log.Debugw("request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", ww.status,
					"duration", duration.String(),
					"remote", r.RemoteAddr,
				)
			}
		})
	}
}

// BodyLimit 对 /api/ 请求体施加大小上限，防止超大 payload 消耗内存。
// overrides 为路径前缀到更大限额的映射（如应用部署文件上传）。
func BodyLimit(limit int64, overrides map[string]int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && strings.HasPrefix(r.URL.Path, "/api/") {
				bodyLimit := limit
				for prefix, l := range overrides {
					if strings.HasPrefix(r.URL.Path, prefix) {
						bodyLimit = l
						break
					}
				}
				r.Body = http.MaxBytesReader(w, r.Body, bodyLimit)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Recovery 捕获 handler panic，返回 500 并记录堆栈。
func Recovery(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Errorw("panic recovered",
						"error", fmt.Sprintf("%v", rec),
						"stack", string(debug.Stack()),
						"path", r.URL.Path,
					)
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprint(w, `{"code":500,"message":"internal server error"}`)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders 添加基础安全头。
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// statusWriter 包装 ResponseWriter 以捕获写入的状态码。
// 同时保留 Hijacker 接口，使 WebSocket 升级可用。
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Hijack 将连接控制权交给调用者（WebSocket 等协议升级需要）。
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("response writer does not support hijack")
}
