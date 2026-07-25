package controller

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type KeepAliveConfig struct {
	Enabled       bool `json:"enabled"`
	Interval      int  `json:"interval_seconds"`
	IdleThreshold int  `json:"idle_threshold_seconds"`
}

type ConfigData struct {
	KeepAlive KeepAliveConfig   `json:"keep_alive"`
	LogPath   string            `json:"log_path,omitempty"`
	Auth      *HybridAuthConfig `json:"auth,omitempty"`
}

type Config struct {
	configData *ConfigData
	configMu   sync.Mutex
	Log        *Log
	logFile    *os.File
}

func NewConfig() *Config {
	c := &Config{}
	c.loadConfigJSON()
	c.Log = NewLog(c.getLogBaseDir())
	return c
}

func (c *Config) loadConfigJSON() {
	c.configMu.Lock()
	defer c.configMu.Unlock()

	if c.configData != nil {
		return
	}

	c.configData = &ConfigData{
		KeepAlive: KeepAliveConfig{
			Enabled:       true,
			Interval:      60,
			IdleThreshold: 30,
		},
		LogPath: "",
		Auth:    nil,
	}

	data, err := os.ReadFile("config.json")
	if err != nil {
		return
	}

	if err := json.Unmarshal(data, c.configData); err != nil {
		return
	}
}

func (c *Config) getLogBaseDir() string {
	c.configMu.Lock()
	defer c.configMu.Unlock()

	if c.configData != nil && c.configData.LogPath != "" {
		expandedPath := os.ExpandEnv(c.configData.LogPath)
		if !filepath.IsAbs(expandedPath) {
			if absPath, err := filepath.Abs(expandedPath); err == nil {
				expandedPath = absPath
			}
		}
		return expandedPath
	}

	return os.TempDir()
}

func (c *Config) GetKeepAliveConfig() KeepAliveConfig {
	c.configMu.Lock()
	defer c.configMu.Unlock()
	return c.configData.KeepAlive
}

func (c *Config) GetHybridAuthConfig() HybridAuthConfig {
	c.configMu.Lock()
	defer c.configMu.Unlock()

	if c.configData.Auth == nil {
		return HybridAuthConfig{}
	}
	return *c.configData.Auth
}

func (c *Config) Close() {
	if c.logFile != nil {
		c.logFile.Close()
	}
}
