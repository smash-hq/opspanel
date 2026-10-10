// Package api 提供 HTTP API 处理器与通用响应封装。
package api

import (
	"encoding/json"
	"net/http"
)

// Response 是统一的 API 响应信封。
type Response struct {
	Code    int    `json:"code"`              // 0 = 成功，非 0 = 业务错误
	Message string `json:"message"`           // 可读提示
	Data    any    `json:"data,omitempty"`    // 业务数据
}

// JSON 写入统一 JSON 响应。
func JSON(w http.ResponseWriter, status int, resp Response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// OK 返回成功响应。
func OK(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: data})
}

// Fail 返回错误响应。
func Fail(w http.ResponseWriter, status int, code int, msg string) {
	JSON(w, status, Response{Code: code, Message: msg})
}
