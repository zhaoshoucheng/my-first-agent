package config

import (
	"fmt"
	"os"
	"sync/atomic"

	"gopkg.in/yaml.v3"
)

var configValue atomic.Pointer[Settings]

// GetConfig 安全地获取当前配置的副本
func GetConfig() *Settings {
	return configValue.Load()
}

func SetConfig(config *Settings) {
	configValue.Store(config)
}

// Init 读取并解析指定路径的 yaml 配置文件，存入包内全局单例。
func Init(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config.Init: read %q: %w", path, err)
	}
	var n yaml.Node
	if err := yaml.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("config.Init: parse %q: %w", path, err)
	}
	var s Settings
	if err := yaml.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("config.Init: decode Settings %q: %w", path, err)
	}
	SetConfig(&s)
	return nil
}
