package api

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// 语言环境安装/卸载任务：异步执行时在内存中记录状态与输出日志，
// 前端通过 /languages/job 轮询展示实时进度。任务仅保留每个 key 最近一次。

const langJobLogMax = 64 << 10 // 单任务日志上限（字节），超出后丢弃头部

// langJob 一次语言环境安装/卸载任务。
type langJob struct {
	action string // install | uninstall

	mu         sync.Mutex
	status     string // running | success | error
	log        strings.Builder
	errMsg     string
	startedAt  time.Time
	finishedAt time.Time
}

// append 追加一块输出，超出上限时保留后半段并打截断标记。
func (j *langJob) append(chunk string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.log.Len()+len(chunk) > langJobLogMax {
		kept := j.log.String()
		if len(kept) > langJobLogMax/2 {
			kept = kept[len(kept)-langJobLogMax/2:]
		}
		j.log.Reset()
		j.log.WriteString("...[日志过长，早期内容已截断]...\n")
		j.log.WriteString(kept)
	}
	j.log.WriteString(chunk)
}

// finish 标记任务结束。err 为 nil 表示成功。
func (j *langJob) finish(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if err != nil {
		j.status = "error"
		j.errMsg = err.Error()
		j.log.WriteString("\n==> 任务失败: " + err.Error() + "\n")
	} else {
		j.status = "success"
		j.log.WriteString("\n==> 任务完成\n")
	}
	j.finishedAt = time.Now()
}

// snapshot 返回任务当前状态的只读视图。
func (j *langJob) snapshot() map[string]any {
	j.mu.Lock()
	defer j.mu.Unlock()
	view := map[string]any{
		"action":     j.action,
		"status":     j.status,
		"log":        j.log.String(),
		"error":      j.errMsg,
		"started_at": j.startedAt.Unix(),
	}
	if !j.finishedAt.IsZero() {
		view["finished_at"] = j.finishedAt.Unix()
	}
	return view
}

// langJobRegistry 任务注册表：key 为 server|lang|action。
type langJobRegistry struct {
	mu   sync.Mutex
	jobs map[string]*langJob
}

var langJobs = &langJobRegistry{jobs: make(map[string]*langJob)}

func langJobKey(serverID, langID, action string) string {
	return serverID + "|" + langID + "|" + action
}

// start 为 key 创建新任务（覆盖旧任务），并写入命令行首行。
func (r *langJobRegistry) start(serverID, langID, action, displayCmd string) *langJob {
	job := &langJob{
		action:    action,
		status:    "running",
		startedAt: time.Now(),
	}
	job.append("$ " + displayCmd + "\n")
	r.mu.Lock()
	r.jobs[langJobKey(serverID, langID, action)] = job
	r.mu.Unlock()
	return job
}

// get 查询任务；不存在（服务重启或未执行过）返回 nil。
func (r *langJobRegistry) get(serverID, langID, action string) (*langJob, error) {
	r.mu.Lock()
	job, ok := r.jobs[langJobKey(serverID, langID, action)]
	r.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("任务不存在")
	}
	return job, nil
}
