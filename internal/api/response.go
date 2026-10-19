// Package api 提供 HTTP API 处理器与通用响应封装。
package api

import (
	"encoding/json"
	"net/http"
)

// Response 是统一的 API 响应信封。
type Response struct {
	Code    int    `json:"code"`             // 0 = 成功，非 0 = 业务错误（与 HTTP 状态码一致）
	Message string `json:"message"`          // 面向用户的规范化中文提示
	Detail  string `json:"detail,omitempty"` // 底层原始错误（err.Error()、命令 stderr 等），供运维排障
	Data    any    `json:"data,omitempty"`   // 业务数据
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

// Fail 返回错误响应：msg 为规范化提示，不携带底层错误串。
// 有 error 对象时请改用 FailErr，将原始错误透出到 detail。
func Fail(w http.ResponseWriter, status int, code int, msg string) {
	JSON(w, status, Response{Code: code, Message: msg})
}

// FailErr 返回带底层错误详情的错误响应：
// message 为面向用户的规范化中文提示，err.Error() 作为 detail 原样透出。
// 运维平台必须让操作者看到失败的真实原因（连接拒绝、权限不足、命令 stderr 等）。
func FailErr(w http.ResponseWriter, status int, msg string, err error) {
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	JSON(w, status, Response{Code: status, Message: msg, Detail: detail})
}
