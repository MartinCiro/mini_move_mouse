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

// Start inicia el bucle de mantenimiento de sesión en segundo plano
func (sm *SessionManager) Start() {
	kaConfig := sm.config.GetKeepAliveConfig()

	if !kaConfig.Enabled {
		sm.config.Log.Comentario("INFO", "🚫 Keep-Alive deshabilitado en la configuración.")
		return
	}

	sm.isRunning = true
	interval := time.Duration(kaConfig.Interval) * time.Second
	ticker := time.NewTicker(interval)

	sm.config.Log.Comentario("INFO", fmt.Sprintf("🖱️  Keep-Alive iniciado. Movimiento cada %d segundos.", kaConfig.Interval))

	// Goroutine para manejar el bucle
	go func() {
		for {
			select {
			case <-ticker.C:
				sm.performMicroMovement()
			case <-sm.stopChan:
				ticker.Stop()
				sm.isRunning = false
				sm.config.Log.Comentario("INFO", "🛑 Keep-Alive detenido.")
				return
			}
		}
	}()
}

// Stop detiene el bucle de forma segura
func (sm *SessionManager) Stop() {
	if sm.isRunning {
		sm.stopChan <- struct{}{}
	}
}

// performMicroMovement ejecuta la lógica de mover y regresar el mouse
func (sm *SessionManager) performMicroMovement() {
	pos, err := GetCursorPosition()
	if err != nil {
		sm.config.Log.Comentario("WARNING", fmt.Sprintf("⚠️ No se pudo obtener la posición del mouse: %v", err))
		return
	}

	// 1. Mover 1 pixel (derecha y abajo)
	_ = SetCursorPosition(pos.X+1, pos.Y+1)

	// 2. Pausa mínima para que el SO registre el evento
	time.Sleep(10 * time.Millisecond)

	// 3. Regresar a la posición original
	_ = SetCursorPosition(pos.X, pos.Y)

	sm.config.Log.Comentario("SUCCESS", "✅ Sesión mantenida activa")
}

// WaitForShutdown bloquea la ejecución principal hasta recibir una señal de cierre
func (sm *SessionManager) WaitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	sm.config.Log.Comentario("INFO", "📥 Señal de terminación recibida.")
}
