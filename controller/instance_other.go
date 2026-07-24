//go:build !windows

package controller

import (
	"fmt"
	"runtime"
)

type InstanceLock struct{}

func NewInstanceLock() *InstanceLock {
	return &InstanceLock{}
}

func (il *InstanceLock) Acquire(log *Log) (bool, error) {
	return false, fmt.Errorf("instancia única no soportada en %s", runtime.GOOS)
}

func (il *InstanceLock) Release(log *Log) {
	// No-op en otras plataformas
}
