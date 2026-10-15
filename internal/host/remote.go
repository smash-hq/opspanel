package host

import (
	"github.com/smash-hq/opspanel/internal/remote"
)

// RemoteCollector 是 remote.RemoteCollector 的类型别名，保持向后兼容。
type RemoteCollector = remote.RemoteCollector

// NewRemoteCollector 创建远程采集器。
var NewRemoteCollector = remote.NewRemoteCollector
