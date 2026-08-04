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

// Log sistema de logging con escritura thread-safe y creación perezosa (lazy)
type Log struct {
	basePath        string
	rutaProcesos    string
	rutaErrores     string
	archivoProcesos string
	archivoErrores  string
	mu              sync.Mutex
}

// NewLog crea una nueva instancia de Log.
// Elimina los logs anteriores al iniciar y crea las carpetas solo cuando se van a usar.
func NewLog(basePath string) *Log {
	if basePath == "" {
		basePath = os.TempDir()
	}

	rutaLogs := filepath.Join(basePath, "logs")

	// 1. ELIMINAR LOGS ANTERIORES AL INICIAR
	// Eliminamos la carpeta 'logs' completa dentro del basePath para empezar limpio
	err := os.RemoveAll(rutaLogs)

	// 2. FALLBACK: Si falla (por permisos en %TEMP%), usar la carpeta del ejecutable
	if err != nil {
		exeDir, _ := os.Executable()
		basePath = filepath.Dir(exeDir)
		rutaLogs = filepath.Join(basePath, "logs")

		// Intentar eliminar también en la carpeta fallback por si acaso
		_ = os.RemoveAll(rutaLogs)
	}

	fechaActual := time.Now().Format("2006-01-02")

	l := &Log{
		basePath:        basePath,
		rutaProcesos:    filepath.Join(basePath, "logs", "procesos"),
		rutaErrores:     filepath.Join(basePath, "logs", "errores"),
		archivoProcesos: filepath.Join(basePath, "logs", "procesos", fmt.Sprintf("LogProcesos_%s.txt", fechaActual)),
		archivoErrores:  filepath.Join(basePath, "logs", "errores", fmt.Sprintf("LogErrores_%s.txt", fechaActual)),
	}

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

// asegurarCarpeta crea la carpeta de forma perezosa (lazy) solo si no existe
func (l *Log) asegurarCarpeta(rutaCarpeta string) {
	if _, err := os.Stat(rutaCarpeta); os.IsNotExist(err) {
		_ = os.MkdirAll(rutaCarpeta, 0755)
	}
}

func (l *Log) escribirLog(archivo string, rutaCarpeta string, mensaje string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 🧹 CREACIÓN PEREZOSA: Solo creamos la carpeta en el momento exacto de escribir
	l.asegurarCarpeta(rutaCarpeta)

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
	l.escribirLog(l.archivoProcesos, l.rutaProcesos, mensaje)
}

func (l *Log) FinProceso(nombreAplicacion ...string) {
	nombre := "Proceso"
	if len(nombreAplicacion) > 0 && nombreAplicacion[0] != "" {
		nombre = nombreAplicacion[0]
	}
	mensaje := l.formatearMensaje(
		fmt.Sprintf("FIN DE EJECUCIÓN - %s - %s", nombre, l.tiempoActual()),
	)
	l.escribirLog(l.archivoProcesos, l.rutaProcesos, mensaje)
}

func (l *Log) Proceso(nombreProceso string) {
	mensaje := fmt.Sprintf("| Ejecutando: %-80s | Hora: %s |", nombreProceso, l.tiempoActual())
	l.escribirLog(l.archivoProcesos, l.rutaProcesos, mensaje)
}

func (l *Log) Comentario(nivel string, mensaje string) {
	contenido := l.formatearMensaje(
		fmt.Sprintf("%s: %s", strings.ToUpper(nivel), mensaje),
		fmt.Sprintf("Hora: %s", l.tiempoActual()),
	)
	l.escribirLog(l.archivoProcesos, l.rutaProcesos, contenido)
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

	// Escribe en errores (aquí se creará la carpeta 'errores' por primera y única vez si hay errores)
	l.escribirLog(l.archivoErrores, l.rutaErrores, contenido)

	// También lo refleja en procesos
	l.escribirLog(l.archivoProcesos, l.rutaProcesos, contenido)
}

func (l *Log) Separador() {
	l.escribirLog(l.archivoProcesos, l.rutaProcesos, strings.Repeat("=", logAncho))
}
