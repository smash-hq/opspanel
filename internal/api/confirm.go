package api

import (
	"net/http"
)

// 危险操作二次确认机制（后端强制）：
// 前端弹窗仅是体验层，真正的防线在服务端 —— 所有破坏性操作
// （删除、终止进程、卸载、服务启停、改写远程配置等）必须显式携带确认标记，
// 否则一律拒绝。防止误触、脚本误用以及不带标记的跨站伪造请求。
//
// 约定：
//   - 有 JSON body 的 POST/PUT：body 增加 "confirm": true 字段；
//   - 无 body 的 DELETE / POST：查询参数 ?confirm=true。

// msgConfirmRequired 缺少二次确认时的统一提示。
const msgConfirmRequired = "危险操作需要二次确认：请在请求中携带 confirm=true 后重试"

// requireBodyConfirm 校验 JSON body 中的 confirm 字段（请求结构体需含
// Confirm bool `json:"confirm"`）。未确认时写入 400 响应并返回 false。
func requireBodyConfirm(w http.ResponseWriter, confirmed bool) bool {
	if confirmed {
		return true
	}
	Fail(w, http.StatusBadRequest, http.StatusBadRequest, msgConfirmRequired)
	return false
}

// requireQueryConfirm 校验查询参数 ?confirm=true（无 body 的危险操作）。
// 未确认时写入 400 响应并返回 false。
func requireQueryConfirm(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Query().Get("confirm") == "true" {
		return true
	}
	Fail(w, http.StatusBadRequest, http.StatusBadRequest, msgConfirmRequired)
	return false
}
