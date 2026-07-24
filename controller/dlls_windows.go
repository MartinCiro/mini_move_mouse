//go:build windows

package controller

import "golang.org/x/sys/windows"

// ============================================================
// DLLs del sistema Windows
// ============================================================
var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
)

// ============================================================
// Procedimientos de user32.dll
// ============================================================
var (
	procKeybdEvent       = user32.NewProc("keybd_event")
	procGetLastInputInfo = user32.NewProc("GetLastInputInfo")
)

// ============================================================
// Procedimientos de kernel32.dll
// ============================================================
var (
	procCreateMutex      = kernel32.NewProc("CreateMutexW")
	procOpenProcess      = kernel32.NewProc("OpenProcess")
	procTerminateProcess = kernel32.NewProc("TerminateProcess")
	procCloseHandle      = kernel32.NewProc("CloseHandle")
	procGetTickCount     = kernel32.NewProc("GetTickCount")
)
