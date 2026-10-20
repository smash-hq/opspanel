package api

import (
	"fmt"
	"sync"
	"time"
)

// pullJobRegistry 镜像拉取任务注册表：key 为 server|image，仅保留每个 key 最近一次任务。
// 复用 langJob 的状态/日志结构（同包），前端通过 /docker/pull/job 轮询展示进度。
type pullJobRegistry struct {
	mu   sync.Mutex
	jobs map[string]*langJob
}

var pullJobs = &pullJobRegistry{jobs: make(map[string]*langJob)}

func pullJobKey(serverID, image string) string {
	return serverID + "|" + image
}

func (r *pullJobRegistry) start(serverID, image, displayCmd string) *langJob {
	job := &langJob{action: "pull", status: "running", startedAt: time.Now()}
	job.append("$ " + displayCmd + "\n")
	r.mu.Lock()
	r.jobs[pullJobKey(serverID, image)] = job
	r.mu.Unlock()
	return job
}

func (r *pullJobRegistry) get(serverID, image string) (*langJob, error) {
	r.mu.Lock()
	job, ok := r.jobs[pullJobKey(serverID, image)]
	r.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("任务不存在")
	}
	return job, nil
}
