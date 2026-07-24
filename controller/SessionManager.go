package controller

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type SessionManager struct {
	config    *Config
	isRunning bool
	stopChan  chan struct{}
}

func NewSessionManager(config *Config) *SessionManager {
	return &SessionManager{
		config:   config,
		stopChan: make(chan struct{}),
	}
}

func (sm *SessionManager) Start() {
	kaConfig := sm.config.GetKeepAliveConfig()

	if !kaConfig.Enabled {
		sm.config.Log.Comentario("INFO", "🚫 Deshabilitado en la configuración.")
		return
	}

	sm.isRunning = true
	interval := time.Duration(kaConfig.Interval) * time.Second
	ticker := time.NewTicker(interval)

	sm.config.Log.Comentario("INFO", fmt.Sprintf("⌨️  Iniciado. Intervalo: %ds, Umbral de inactividad: %ds",
		kaConfig.Interval, kaConfig.IdleThreshold))

	go func() {
		for {
			select {
			case <-ticker.C:
				sm.performKeyPress()
			case <-sm.stopChan:
				ticker.Stop()
				sm.isRunning = false
				sm.config.Log.Comentario("INFO", "🛑 Detenido.")
				return
			}
		}
	}()
}

func (sm *SessionManager) Stop() {
	if sm.isRunning {
		sm.stopChan <- struct{}{}
	}
}

func (sm *SessionManager) performKeyPress() {
	kaConfig := sm.config.GetKeepAliveConfig()

	// 1. Verificar tiempo de inactividad del usuario
	idleTimeMs, err := GetIdleTime()
	if err != nil {
		sm.config.Log.Comentario("WARNING", fmt.Sprintf("⚠️ No se pudo obtener tiempo de inactividad: %v", err))
		return
	}

	idleTimeSec := idleTimeMs / 1000
	threshold := uint32(kaConfig.IdleThreshold)

	// 2. Si el usuario ha estado activo hace menos de 30 segundos, NO presionar la tecla
	if idleTimeSec < threshold {
		sm.config.Log.Comentario("INFO", fmt.Sprintf("⏸️  Usuario activo (inactivo hace %ds). Pausando bot.", idleTimeSec))
		return
	}

	// 3. Usuario inactivo: presionar la tecla
	err = SimulateNumpad5KeyPress()
	if err != nil {
		sm.config.Log.Comentario("WARNING", fmt.Sprintf("⚠️ No se pudo simular tecla: %v", err))
		return
	}

	sm.config.Log.Comentario("SUCCESS", fmt.Sprintf("✅ Tecla NumPad5 presionada - Sesión activa (usuario inactivo hace %ds)", idleTimeSec))
}

func (sm *SessionManager) WaitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	sm.config.Log.Comentario("INFO", "📥 Señal de terminación recibida.")
}
