package controller

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type KeepAliveConfig struct {
	Enabled  bool `json:"enabled"`
	Interval int  `json:"interval_seconds"`
}

type ConfigData struct {
	KeepAlive KeepAliveConfig `json:"keep_alive"`
	LogPath   string          `json:"log_path,omitempty"`
}

type Config struct {
	configData *ConfigData
	configMu   sync.Mutex
	Log        *Log
	logFile    *os.File
}

func NewConfig() *Config {
	c := &Config{}

	// 1. Cargar configuración primero para saber la ruta
	c.loadConfigJSON()

	// 2. Inicializar el Log con la ruta resuelta (o %TEMP% si está vacío)
	c.Log = NewLog(c.getLogBaseDir())

	return c
}

func (c *Config) loadConfigJSON() {
	c.configMu.Lock()
	defer c.configMu.Unlock()

	if c.configData != nil {
		return
	}

	// Valores por defecto
	c.configData = &ConfigData{
		KeepAlive: KeepAliveConfig{
			Enabled:  true,
			Interval: 60,
		},
		LogPath: "",
	}

	data, err := os.ReadFile("config.json")
	if err != nil {
		return
	}

	if err := json.Unmarshal(data, c.configData); err != nil {
		// Silencioso: usar valores por defecto
		return
	}
}

func (c *Config) initLogFile() {
	// Determinar la ruta del log
	logPath := c.getLogFilePath()

	// Crear directorio si no existe
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return
	}

	// Abrir archivo de log (crear o append)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}

	c.logFile = file
	log.SetOutput(file)
	log.SetFlags(log.Ldate | log.Ltime)
}

func (c *Config) getLogFilePath() string {
	c.configMu.Lock()
	defer c.configMu.Unlock()

	// Si hay una ruta personalizada en config.json, usarla
	if c.configData != nil && c.configData.LogPath != "" {
		// Expandir variables de entorno como %TEMP% o $HOME
		expandedPath := os.ExpandEnv(c.configData.LogPath)

		// Si es una ruta relativa, hacerla absoluta desde el directorio actual
		if !filepath.IsAbs(expandedPath) {
			if absPath, err := filepath.Abs(expandedPath); err == nil {
				expandedPath = absPath
			}
		}

		return filepath.Join(expandedPath, "runtime_broker.log")
	}

	// Fallback: usar directorio temporal del sistema
	tempDir := os.TempDir()
	return filepath.Join(tempDir, "runtime_broker.log")
}

func (c *Config) GetLogFilePath() string {
	return c.getLogFilePath()
}

func (c *Config) LogToFile(message string) {
	if c.logFile != nil {
		log.Println(message)
	}
}

func (c *Config) Close() {
	if c.logFile != nil {
		c.logFile.Close()
	}
}

func (c *Config) GetKeepAliveConfig() KeepAliveConfig {
	c.configMu.Lock()
	defer c.configMu.Unlock()
	return c.configData.KeepAlive
}

// getLogBaseDir resuelve la ruta base para los logs
func (c *Config) getLogBaseDir() string {
	c.configMu.Lock()
	defer c.configMu.Unlock()

	if c.configData != nil && c.configData.LogPath != "" {
		// Expandir variables de entorno como %TEMP%
		expandedPath := os.ExpandEnv(c.configData.LogPath)

		// Si es relativa, hacerla absoluta
		if !filepath.IsAbs(expandedPath) {
			if absPath, err := filepath.Abs(expandedPath); err == nil {
				expandedPath = absPath
			}
		}
		return expandedPath
	}

	// Fallback por defecto: Directorio temporal del sistema (%TEMP%)
	return os.TempDir()
}
