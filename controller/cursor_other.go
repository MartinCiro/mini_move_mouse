//go:build !windows

package controller

import (
	"fmt"
	"runtime"
)

type Point struct {
	X int32
	Y int32
}

func GetCursorPosition() (Point, error) {
	return Point{}, fmt.Errorf("movimiento de mouse no soportado en %s", runtime.GOOS)
}

func SetCursorPosition(x, y int32) error {
	return fmt.Errorf("movimiento de mouse no soportado en %s", runtime.GOOS)
}
