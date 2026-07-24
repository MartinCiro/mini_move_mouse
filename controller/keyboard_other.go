//go:build !windows

package controller

import (
	"fmt"
	"runtime"
)

func SimulateKeyPress(vkCode uint16) error {
	return fmt.Errorf("simulación de teclas no soportada en %s", runtime.GOOS)
}

func SimulateNumpad5KeyPress() error {
	return fmt.Errorf("simulación de teclas no soportada en %s", runtime.GOOS)
}

func GetIdleTime() (uint32, error) {
	return 0, fmt.Errorf("detección de inactividad no soportada en %s", runtime.GOOS)
}
