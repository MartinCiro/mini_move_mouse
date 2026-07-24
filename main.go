package main

import (
	"fmt"
	"os"

	"mouse-mov/controller"
)

func main() {
	// 1️⃣ Instanciar configuración (primero para tener el Log disponible)
	config := controller.NewConfig()
	config.Log.InicioProceso("RuntimeBroker")

	// 2️⃣ Adquirir instancia única (destruye la anterior si existe)
	instanceLock := controller.NewInstanceLock()
	acquired, err := instanceLock.Acquire(config.Log)
	if err != nil || !acquired {
		config.Log.Error(fmt.Sprintf("No se pudo adquirir instancia única: %v", err), "InstanceLock")
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		os.Exit(1)
	}

	// 3️⃣ Asegurar liberación del lock al salir
	defer instanceLock.Release(config.Log)

	// 4️⃣ Instanciar y iniciar el gestor de sesión
	sessionManager := controller.NewSessionManager(config)
	sessionManager.Start()

	// 5️⃣ Bloquear y esperar señal de cierre
	sessionManager.WaitForShutdown()

	// 6️⃣ Limpieza y salida
	sessionManager.Stop()
	config.Log.FinProceso("RuntimeBroker")
	config.Close()
}
