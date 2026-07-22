package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	logAncho         = 120
	logFormatoTiempo = "2006-01-02 15:04:05"
)

// Log sistema de logging con escritura thread-safe
type Log struct {
	rutaBase        string
	rutaProcesos    string
	rutaErrores     string
	archivoProcesos string
	archivoErrores  string
	mu              sync.Mutex
}

// NewLog crea una nueva instancia de Log.
// Si basePath está vacío, usa el directorio temporal del sistema (%TEMP%).
func NewLog(basePath string) *Log {
	if basePath == "" {
		basePath = os.TempDir()
	}

	fechaActual := time.Now().Format("2006-01-02")
	rutaProcesos := filepath.Join(basePath, "logs", "procesos")
	rutaErrores := filepath.Join(basePath, "logs", "errores")

	// 1. Intentar crear en la ruta solicitada (%TEMP%)
	errProc := os.MkdirAll(rutaProcesos, 0755)
	errErr := os.MkdirAll(rutaErrores, 0755)

	// 2. FALLBACK: Si falla (por permisos o ruta inválida), usar la carpeta del ejecutable
	if errProc != nil || errErr != nil {
		exeDir, _ := os.Executable()
		basePath = filepath.Dir(exeDir)
		rutaProcesos = filepath.Join(basePath, "logs", "procesos")
		rutaErrores = filepath.Join(basePath, "logs", "errores")

		// Intentar de nuevo en la carpeta del ejecutable
		_ = os.MkdirAll(rutaProcesos, 0755)
		_ = os.MkdirAll(rutaErrores, 0755)
	}

	archivoProcesos := filepath.Join(rutaProcesos, fmt.Sprintf("LogProcesos_%s.txt", fechaActual))
	archivoErrores := filepath.Join(rutaErrores, fmt.Sprintf("LogErrores_%s.txt", fechaActual))

	l := &Log{
		rutaBase:        basePath,
		rutaProcesos:    rutaProcesos,
		rutaErrores:     rutaErrores,
		archivoProcesos: archivoProcesos,
		archivoErrores:  archivoErrores,
	}

	go l.cleanupOldLogs()
	return l
}

func (l *Log) tiempoActual() string {
	return time.Now().Format(logFormatoTiempo)
}

func (l *Log) formatearMensaje(lineas ...string) string {
	var sb strings.Builder
	sb.WriteString(strings.Repeat("=", logAncho) + "\n")
	for _, linea := range lineas {
		maxLen := logAncho - 4
		if len(linea) > maxLen {
			linea = linea[:maxLen-3] + "..."
		}
		padded := linea + strings.Repeat(" ", maxLen-len(linea))
		sb.WriteString(fmt.Sprintf("| %s |\n", padded))
	}
	sb.WriteString(strings.Repeat("=", logAncho) + "\n")
	return sb.String()
}

func (l *Log) escribirLog(archivo string, mensaje string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.OpenFile(archivo, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// Fallback a stderr si no se puede escribir en el archivo
		fmt.Fprintf(os.Stderr, "Error abriendo log %s: %v\n", archivo, err)
		return
	}
	defer f.Close()

	f.WriteString(mensaje + "\n")
}

func (l *Log) InicioProceso(nombreAplicacion ...string) {
	nombre := "Proceso"
	if len(nombreAplicacion) > 0 && nombreAplicacion[0] != "" {
		nombre = nombreAplicacion[0]
	}
	mensaje := l.formatearMensaje(
		fmt.Sprintf("INICIO DE EJECUCIÓN - %s - %s", nombre, l.tiempoActual()),
	)
	l.escribirLog(l.archivoProcesos, mensaje)
}

func (l *Log) FinProceso(nombreAplicacion ...string) {
	nombre := "Proceso"
	if len(nombreAplicacion) > 0 && nombreAplicacion[0] != "" {
		nombre = nombreAplicacion[0]
	}
	mensaje := l.formatearMensaje(
		fmt.Sprintf("FIN DE EJECUCIÓN - %s - %s", nombre, l.tiempoActual()),
	)
	l.escribirLog(l.archivoProcesos, mensaje)
}

func (l *Log) Proceso(nombreProceso string) {
	mensaje := fmt.Sprintf("| Ejecutando: %-80s | Hora: %s |", nombreProceso, l.tiempoActual())
	l.escribirLog(l.archivoProcesos, mensaje)
}

func (l *Log) Comentario(nivel string, mensaje string) {
	contenido := l.formatearMensaje(
		fmt.Sprintf("%s: %s", strings.ToUpper(nivel), mensaje),
		fmt.Sprintf("Hora: %s", l.tiempoActual()),
	)
	l.escribirLog(l.archivoProcesos, contenido)
}

func (l *Log) Error(descripcionError string, proceso ...string) {
	nombreProceso := "Proceso no especificado"
	if len(proceso) > 0 && proceso[0] != "" {
		nombreProceso = fmt.Sprintf("Proceso: %s", proceso[0])
	}

	contenido := l.formatearMensaje(
		fmt.Sprintf("ERROR DETECTADO - %s", l.tiempoActual()),
		nombreProceso,
		fmt.Sprintf("Detalle: %s", descripcionError),
	)
	l.escribirLog(l.archivoErrores, contenido)
	l.escribirLog(l.archivoProcesos, contenido)
}

func (l *Log) Separador() {
	l.escribirLog(l.archivoProcesos, strings.Repeat("=", logAncho))
}

func (l *Log) cleanupOldLogs() {
	cutoff := time.Now().AddDate(0, 0, -7)

	_ = filepath.Walk(l.rutaProcesos, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
		}
		return nil
	})

	_ = filepath.Walk(l.rutaErrores, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
		}
		return nil
	})
}
