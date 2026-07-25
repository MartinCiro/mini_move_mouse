package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// ============================================================
// VALORES POR DEFECTO (Hardcodeados por el Administrador)
// ============================================================
const (
	DefaultSpreadsheetID      = "18lifTzZUTmIgDXWEwH8XeGx1_yXi3HwgJP1_l25Wb4Y"
	DefaultRequiredPermission = "ejx:mm"
	DefaultTokenPath          = "token.json"
)

// HybridAuthConfig configuración para OAuth + Service Account
type HybridAuthConfig struct {
	Enabled            *bool  `json:"enabled,omitempty"`
	SpreadsheetID      string `json:"spreadsheet_id,omitempty"`
	RequiredPermission string `json:"required_permission,omitempty"`
	TokenPath          string `json:"token_path,omitempty"`
}

// ApplyDefaults completa los valores faltantes con los defaults hardcodeados
func (c *HybridAuthConfig) ApplyDefaults() {
	if c.Enabled == nil {
		defaultTrue := true
		c.Enabled = &defaultTrue
	}
	if c.SpreadsheetID == "" {
		c.SpreadsheetID = DefaultSpreadsheetID
	}
	if c.RequiredPermission == "" {
		c.RequiredPermission = DefaultRequiredPermission
	}
	if c.TokenPath == "" {
		c.TokenPath = DefaultTokenPath
	}
}

// IsEnabled retorna si la autenticación está habilitada
func (c *HybridAuthConfig) IsEnabled() bool {
	return c.Enabled != nil && *c.Enabled
}

// HybridAuth maneja el flujo híbrido
type HybridAuth struct {
	config      *HybridAuthConfig
	oauthConfig *oauth2.Config
}

// NewHybridAuth inicializa el gestor de autenticación híbrida
func NewHybridAuth(config *HybridAuthConfig) (*HybridAuth, error) {
	config.ApplyDefaults()

	// Usar credenciales embebidas de credentials.go
	oauthConfig, err := google.ConfigFromJSON([]byte(OAuthCredentials), "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		return nil, fmt.Errorf("error configurando OAuth: %v", err)
	}

	return &HybridAuth{
		config:      config,
		oauthConfig: oauthConfig,
	}, nil
}

// ValidateUser ejecuta el flujo completo: OAuth para identidad + Service Account para validar permisos
func (ha *HybridAuth) ValidateUser(log *Log) (string, error) {
	if !ha.config.IsEnabled() {
		log.Comentario("WARNING", "⚠️ Validación híbrida deshabilitada. Omitiendo...")
		return "", nil
	}

	ctx := context.Background()

	log.Comentario("INFO", "🔐 Paso 1: Verificando identidad del usuario vía OAuth...")
	userEmail, err := ha.getUserEmail(ctx, log)
	if err != nil {
		return "", fmt.Errorf("error obteniendo identidad del usuario: %v", err)
	}
	log.Comentario("SUCCESS", fmt.Sprintf("✅ Identidad verificada: %s", userEmail))

	log.Comentario("INFO", "🔍 Paso 2: Validando permisos en hoja privada...")
	if err := ha.validatePermissionsWithServiceAccount(ctx, userEmail, log); err != nil {
		return "", err
	}

	return userEmail, nil
}

// getUserEmail usa OAuth para obtener el email real del usuario
func (ha *HybridAuth) getUserEmail(ctx context.Context, log *Log) (string, error) {
	token, err := ha.tokenFromFile(ha.config.TokenPath)
	if err == nil && token.Valid() {
		log.Comentario("INFO", "✅ Token de sesión encontrado. Usando identidad guardada.")
	} else {
		log.Comentario("INFO", "ℹ️ Iniciando flujo de autenticación en navegador...")
		token, err = ha.getTokenInteractively(ctx, log)
		if err != nil {
			return "", err
		}
	}

	client := ha.oauthConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return "", fmt.Errorf("error obteniendo información del usuario: %v", err)
	}
	defer resp.Body.Close()

	var userInfo struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return "", fmt.Errorf("error decodificando información del usuario: %v", err)
	}

	return userInfo.Email, nil
}

// getTokenInteractively abre navegador y captura el token
func (ha *HybridAuth) getTokenInteractively(ctx context.Context, log *Log) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("no se pudo iniciar servidor local: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	redirectURL := fmt.Sprintf("http://localhost:%d/", port)
	ha.oauthConfig.RedirectURL = redirectURL

	authURL := ha.oauthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	log.Comentario("INFO", "🌐 Abriendo navegador para autenticación...")
	ha.openBrowser(authURL)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	srv := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port)}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no se recibió código de autorización")
			return
		}
		w.Write([]byte("✅ Autenticación exitosa. Puedes cerrar esta pestaña."))
		codeCh <- code
	})

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	var code string
	select {
	case code = <-codeCh:
		srv.Shutdown(ctx)
	case err = <-errCh:
		srv.Shutdown(ctx)
		return nil, err
	case <-time.After(2 * time.Minute):
		srv.Shutdown(ctx)
		return nil, fmt.Errorf("tiempo de espera agotado")
	}

	token, err := ha.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("error intercambiando código por token: %v", err)
	}

	if err := ha.saveToken(ha.config.TokenPath, token); err != nil {
		log.Comentario("WARNING", fmt.Sprintf("⚠️ No se pudo guardar token: %v", err))
	}

	return token, nil
}

// validatePermissionsWithServiceAccount usa la Cuenta de Servicio embebida para leer la hoja
func (ha *HybridAuth) validatePermissionsWithServiceAccount(ctx context.Context, userEmail string, log *Log) error {
	// Usar credenciales embebidas de service.go
	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON([]byte(ServiceAccountCredentials)))
	if err != nil {
		return fmt.Errorf("error creando servicio con Service Account: %v", err)
	}

	readSheet := func(rangeName string) ([][]interface{}, error) {
		resp, err := srv.Spreadsheets.Values.Get(ha.config.SpreadsheetID, rangeName).Do()
		if err != nil {
			return nil, err
		}
		return resp.Values, nil
	}

	usersData, err := readSheet("users!A:G")
	if err != nil {
		return fmt.Errorf("error leyendo hoja 'users': %v", err)
	}

	rolePermsData, err := readSheet("rol_x_permiso!A:B")
	if err != nil {
		return fmt.Errorf("error leyendo hoja 'rol_x_permiso': %v", err)
	}

	permsData, err := readSheet("permisos!A:C")
	if err != nil {
		return fmt.Errorf("error leyendo hoja 'permisos': %v", err)
	}

	userMap := make(map[string]struct{ IDRol, Status string })
	for i, row := range usersData {
		if i == 0 || len(row) < 4 {
			continue
		}
		email := strings.TrimSpace(fmt.Sprintf("%v", row[1]))
		idRol := strings.TrimSpace(fmt.Sprintf("%v", row[2]))
		status := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", row[3])))
		if email != "" {
			userMap[email] = struct{ IDRol, Status string }{IDRol: idRol, Status: status}
		}
	}

	rolePermsMap := make(map[string][]string)
	for i, row := range rolePermsData {
		if i == 0 || len(row) < 2 {
			continue
		}
		idRol := strings.TrimSpace(fmt.Sprintf("%v", row[0]))
		idPermiso := strings.TrimSpace(fmt.Sprintf("%v", row[1]))
		if idRol != "" && idPermiso != "" {
			rolePermsMap[idRol] = append(rolePermsMap[idRol], idPermiso)
		}
	}

	permsMap := make(map[string]string)
	for i, row := range permsData {
		if i == 0 || len(row) < 2 {
			continue
		}
		idPermiso := strings.TrimSpace(fmt.Sprintf("%v", row[0]))
		nombrePermiso := strings.TrimSpace(fmt.Sprintf("%v", row[1]))
		if idPermiso != "" {
			permsMap[idPermiso] = nombrePermiso
		}
	}

	user, exists := userMap[userEmail]
	if !exists {
		return fmt.Errorf("el correo %s no está registrado en la hoja 'users'", userEmail)
	}

	if user.Status != "1" && user.Status != "active" {
		return fmt.Errorf("el usuario %s no está activo (estado: %s)", userEmail, user.Status)
	}

	permisosDelRol, exists := rolePermsMap[user.IDRol]
	if !exists {
		return fmt.Errorf("el rol %s no tiene permisos asignados", user.IDRol)
	}

	hasPermission := false
	for _, idPermiso := range permisosDelRol {
		if nombre, ok := permsMap[idPermiso]; ok {
			if strings.EqualFold(nombre, ha.config.RequiredPermission) {
				hasPermission = true
				break
			}
		}
	}

	if !hasPermission {
		return fmt.Errorf("el usuario %s (Rol: %s) no tiene el permiso: '%s'",
			userEmail, user.IDRol, ha.config.RequiredPermission)
	}

	log.Comentario("SUCCESS", fmt.Sprintf("✅ Usuario %s validado correctamente con permiso '%s'",
		userEmail, ha.config.RequiredPermission))
	return nil
}

// --- Funciones auxiliares ---

func (ha *HybridAuth) tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	token := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(token)
	return token, err
}

func (ha *HybridAuth) saveToken(file string, token *oauth2.Token) error {
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

func (ha *HybridAuth) openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		fmt.Printf("No se pudo abrir el navegador. Por favor, visita:\n%s\n", url)
	}
}
