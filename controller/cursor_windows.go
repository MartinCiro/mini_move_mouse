//go:build windows

package controller

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	procGetCursorPos = user32.NewProc("GetCursorPos")
	procSetCursorPos = user32.NewProc("SetCursorPos")
)

type Point struct {
	X int32
	Y int32
}

// GetCursorPosition obtiene las coordenadas actuales del mouse
func GetCursorPosition() (Point, error) {
	var p Point
	ret, _, err := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	if ret == 0 {
		return p, fmt.Errorf("error al obtener posición: %v", err)
	}
	return p, nil
}

// SetCursorPosition mueve el mouse a las coordenadas X, Y
func SetCursorPosition(x, y int32) error {
	ret, _, err := procSetCursorPos.Call(uintptr(x), uintptr(y))
	if ret == 0 {
		return fmt.Errorf("error al establecer posición: %v", err)
	}
	return nil
}
