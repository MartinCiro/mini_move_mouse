package main

import (
	"fmt"
	"mouse-mov/controller" // Ajusta "go-indeed" al nombre real de tu módulo en go.mod
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("🛡️  Anti-Bloqueo de Sesión (Keep-Alive)")
	fmt.Println("==================================================")

	// 1️⃣ Instanciar configuración
	config := controller.NewConfig()
	config.Log.InicioProceso("KeepAliveBot")

	// 2️⃣ Instanciar y iniciar el gestor de sesión
	sessionManager := controller.NewSessionManager(config)
	sessionManager.Start()

	// 3️⃣ Bloquear y esperar señal de cierre (Ctrl+C)
	sessionManager.WaitForShutdown()

	// 4️⃣ Limpieza y salida
	sessionManager.Stop()
	config.Log.FinProceso("KeepAliveBot")
	fmt.Println("✅ Programa finalizado correctamente.")
}
