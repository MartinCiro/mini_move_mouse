package main

import (
	"fmt"
	"os"

	"mouse-mov/controller"
)

func main() {
	config := controller.NewConfig()
	config.Log.InicioProceso("RuntimeBroker")

	instanceLock := controller.NewInstanceLock()
	acquired, err := instanceLock.Acquire(config.Log)
	if err != nil || !acquired {
		config.Log.Error(fmt.Sprintf("No se pudo adquirir instancia única: %v", err), "InstanceLock")
		os.Exit(1)
	}
	defer instanceLock.Release(config.Log)

	authConfig := config.GetHybridAuthConfig()
	auth, err := controller.NewHybridAuth(&authConfig)
	if err != nil {
		config.Log.Error(fmt.Sprintf("Error inicializando autenticación híbrida: %v", err), "HybridAuth")
		os.Exit(1)
	}

	userEmail, err := auth.ValidateUser(config.Log)
	if err != nil {
		config.Log.Error(fmt.Sprintf("ACCESO DENEGADO: %v", err), "HybridAuth")
		fmt.Fprintf(os.Stderr, "❌ Error de autorización: %v\n", err)
		os.Exit(1)
	}

	if userEmail != "" {
		config.Log.Comentario("INFO", fmt.Sprintf("👤 Sesión iniciada como: %s", userEmail))
	}

	sessionManager := controller.NewSessionManager(config)
	sessionManager.Start()
	sessionManager.WaitForShutdown()
	sessionManager.Stop()

	config.Log.FinProceso("RuntimeBroker")
	config.Close()
}
