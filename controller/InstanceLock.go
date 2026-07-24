//go:build windows

package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	mutexName = "Global\\RuntimeBroker_SingleInstance_Mutex"
	pidFile   = "runtime_broker.pid"
)

const (
	PROCESS_TERMINATE = 0x0001
)

// InstanceLock maneja la exclusividad de instancia del bot
type InstanceLock struct {
	handle  windows.Handle
	pidPath string
}

// NewInstanceLock crea un nuevo gestor de instancia única
func NewInstanceLock() *InstanceLock {
	return &InstanceLock{
		pidPath: filepath.Join(os.TempDir(), pidFile),
	}
}

// Acquire intenta adquirir el lock. Si hay una instancia anterior, la destruye.
func (il *InstanceLock) Acquire(log *Log) (bool, error) {
	mutexNamePtr, err := syscall.UTF16PtrFromString(mutexName)
	if err != nil {
		return false, fmt.Errorf("error creando nombre del mutex: %v", err)
	}

	handle, _, lastErr := procCreateMutex.Call(
		0,
		0,
		uintptr(unsafe.Pointer(mutexNamePtr)),
	)

	if handle == 0 {
		return false, fmt.Errorf("error creando mutex: %v", lastErr)
	}

	il.handle = windows.Handle(handle)

	// ERROR_ALREADY_EXISTS = 183
	if lastErr == syscall.Errno(183) {
		if log != nil {
			log.Comentario("WARNING", "⚠️ Ya existe una instancia del bot. Intentando detenerla...")
		}

		il.killPreviousInstance(log)

		// ✅ FIX: Usar time.Sleep en lugar de syscall.Sleep
		for i := 0; i < 10; i++ {
			procCloseHandle.Call(uintptr(il.handle))

			handle, _, lastErr = procCreateMutex.Call(
				0,
				0,
				uintptr(unsafe.Pointer(mutexNamePtr)),
			)

			if handle != 0 && lastErr != syscall.Errno(183) {
				il.handle = windows.Handle(handle)
				break
			}

			// ✅ FIX: time.Sleep en lugar de syscall.Sleep
			time.Sleep(200 * time.Millisecond)
		}

		if lastErr == syscall.Errno(183) {
			return false, fmt.Errorf("no se pudo adquirir el mutex: otra instancia sigue activa")
		}
	}

	if err := il.saveCurrentPID(); err != nil {
		if log != nil {
			log.Comentario("WARNING", fmt.Sprintf("⚠️ No se pudo guardar PID: %v", err))
		}
	}

	if log != nil {
		log.Comentario("SUCCESS", fmt.Sprintf("🔒 Instancia única adquirida (PID: %d)", os.Getpid()))
	}

	return true, nil
}

// Release libera el mutex y limpia el archivo PID
func (il *InstanceLock) Release(log *Log) {
	if il.handle != 0 {
		procCloseHandle.Call(uintptr(il.handle))
		il.handle = 0
	}

	if err := os.Remove(il.pidPath); err != nil && !os.IsNotExist(err) {
		if log != nil {
			log.Comentario("WARNING", fmt.Sprintf("⚠️ No se pudo eliminar archivo PID: %v", err))
		}
	}

	if log != nil {
		log.Comentario("INFO", "🔓 Instancia única liberada")
	}
}

func (il *InstanceLock) saveCurrentPID() error {
	pid := os.Getpid()
	return os.WriteFile(il.pidPath, []byte(strconv.Itoa(pid)), 0644)
}

func (il *InstanceLock) readPreviousPID() (int, error) {
	data, err := os.ReadFile(il.pidPath)
	if err != nil {
		return 0, err
	}

	pidStr := strings.TrimSpace(string(data))
	return strconv.Atoi(pidStr)
}

func (il *InstanceLock) killPreviousInstance(log *Log) {
	prevPID, err := il.readPreviousPID()
	if err != nil {
		if log != nil {
			log.Comentario("WARNING", fmt.Sprintf("⚠️ No se pudo leer PID anterior: %v", err))
		}
		return
	}

	if prevPID == os.Getpid() {
		return
	}

	if log != nil {
		log.Comentario("INFO", fmt.Sprintf("🎯 Intentando detener instancia anterior (PID: %d)...", prevPID))
	}

	hProcess, _, err := procOpenProcess.Call(
		PROCESS_TERMINATE,
		0,
		uintptr(prevPID),
	)

	if hProcess == 0 {
		if log != nil {
			log.Comentario("WARNING", fmt.Sprintf("⚠️ No se pudo abrir proceso %d: %v", prevPID, err))
		}
		return
	}

	ret, _, err := procTerminateProcess.Call(hProcess, 0)
	procCloseHandle.Call(hProcess)

	if ret == 0 {
		if log != nil {
			log.Comentario("WARNING", fmt.Sprintf("⚠️ No se pudo terminar proceso %d: %v", prevPID, err))
		}
		return
	}

	if log != nil {
		log.Comentario("SUCCESS", fmt.Sprintf("✅ Instancia anterior (PID: %d) detenida correctamente", prevPID))
	}
}
