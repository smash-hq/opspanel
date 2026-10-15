// Package config 负责应用配置的加载与默认值。
// 使用 Viper 支持配置文件、环境变量与命令行标志三种来源。
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/spf13/viper"
)

// Config 是应用运行期的完整配置结构。
type Config struct {
	Server ServerConfig `mapstructure:"server"`
	Log    LogConfig    `mapstructure:"log"`
}

// ServerConfig 控制 HTTP/WebSocket 服务行为。
type ServerConfig struct {
	Host            string `mapstructure:"host"`             // 监听地址，默认 0.0.0.0
	Port            int    `mapstructure:"port"`             // 监听端口，默认 18080
	ShutdownTimeout int    `mapstructure:"shutdown_timeout"` // 优雅关闭超时秒数，默认 10
	PushInterval    int    `mapstructure:"push_interval"`    // WebSocket 推送间隔秒数，默认 3
	DataDir         string `mapstructure:"data_dir"`         // 数据存储目录，默认 ./data
}

// LogConfig 控制日志行为。
type LogConfig struct {
	Level  string `mapstructure:"level"`  // debug / info / warn / error
	Format string `mapstructure:"format"` // console / json
}

// Default 返回零值安全的默认配置。
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            18080,
			ShutdownTimeout: 10,
			PushInterval:    3,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "console",
		},
	}
}

// Load 从配置文件（可选）与环境变量合并出最终配置。
// 配置文件缺失不报错，仅使用默认值与环境变量。
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	// 设置默认值
	def := Default()
	v.SetDefault("server.host", def.Server.Host)
	v.SetDefault("server.port", def.Server.Port)
	v.SetDefault("server.shutdown_timeout", def.Server.ShutdownTimeout)
	v.SetDefault("server.push_interval", def.Server.PushInterval)
	v.SetDefault("server.data_dir", def.Server.DataDir)
	v.SetDefault("log.level", def.Log.Level)
	v.SetDefault("log.format", def.Log.Format)

	// 环境变量：OPSPANEL_LOG_LEVEL、OPSPANEL_SERVER_PORT 等
	v.SetEnvPrefix("OPSPANEL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 配置文件
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/opspanel/")
	}
	if err := v.ReadInConfig(); err != nil {
		// 配置文件缺失是正常情况；存在但解析失败则直接报错，避免静默回退到默认值
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("read config file: %w", err)
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
