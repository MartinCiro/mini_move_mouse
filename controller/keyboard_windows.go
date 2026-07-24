//go:build windows

package controller

import (
	"fmt"
	"unsafe"
)

const (
	VK_F13          = 0x7C
	KEYEVENTF_KEYUP = 0x0002
)

// LASTINPUTINFO estructura para GetLastInputInfo
type LASTINPUTINFO struct {
	cbSize uint32
	dwTime uint32
}

// SimulateKeyPress simula la presión y liberación de una tecla
func SimulateKeyPress(vkCode uint16) error {
	// 1. Presionar la tecla (key down)
	ret, _, err := procKeybdEvent.Call(
		uintptr(vkCode),
		uintptr(0),
		uintptr(0),
		uintptr(0),
	)
	if ret == 0 {
		return fmt.Errorf("error al presionar tecla: %v", err)
	}

	// 2. Liberar la tecla (key up)
	ret, _, err = procKeybdEvent.Call(
		uintptr(vkCode),
		uintptr(0),
		uintptr(KEYEVENTF_KEYUP),
		uintptr(0),
	)
	if ret == 0 {
		return fmt.Errorf("error al liberar tecla: %v", err)
	}

	return nil
}

// SimulateNumpad5KeyPress simula específicamente la tecla "5" del teclado numérico
func SimulateNumpad5KeyPress() error {
	return SimulateKeyPress(VK_F13)
}

// GetTickCount retorna el tiempo en milisegundos desde que se inició Windows
func GetTickCount() uint32 {
	ret, _, _ := procGetTickCount.Call()
	return uint32(ret)
}

// GetIdleTime retorna el tiempo en milisegundos desde la última actividad del usuario
func GetIdleTime() (uint32, error) {
	var lii LASTINPUTINFO
	lii.cbSize = uint32(unsafe.Sizeof(lii))

	ret, _, err := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lii)))
	if ret == 0 {
		return 0, fmt.Errorf("error al obtener última actividad: %v", err)
	}

	currentTime := GetTickCount()
	idleTime := currentTime - lii.dwTime

	return idleTime, nil
}
