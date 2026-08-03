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

// AuthConfig configuración para OAuth + Lectura directa de Sheets
type AuthConfig struct {
	Enabled            *bool  `json:"enabled,omitempty"`
	SpreadsheetID      string `json:"spreadsheet_id,omitempty"`
	RequiredPermission string `json:"required_permission,omitempty"`
	TokenPath          string `json:"token_path,omitempty"`
}

func (c *AuthConfig) ApplyDefaults() {
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

func (c *AuthConfig) IsEnabled() bool {
	return c.Enabled != nil && *c.Enabled
}

// UserAuth maneja el flujo de autenticación
type UserAuth struct {
	config      *AuthConfig
	oauthConfig *oauth2.Config
}

func NewUserAuth(config *AuthConfig) (*UserAuth, error) {
	config.ApplyDefaults()

	// ✅ USAR CREDENCIALES EMBEBIDAS de credentials.go (NO se lee archivo externo)
	oauthConfig, err := google.ConfigFromJSON([]byte(OAuthCredentials),
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/spreadsheets.readonly")
	if err != nil {
		return nil, fmt.Errorf("error configurando OAuth: %v", err)
	}

	return &UserAuth{
		config:      config,
		oauthConfig: oauthConfig,
	}, nil
}

func (ua *UserAuth) ValidateUser(log *Log) (string, error) {
	if !ua.config.IsEnabled() {
		log.Comentario("WARNING", "⚠️ Validación deshabilitada. Omitiendo...")
		return "", nil
	}

	ctx := context.Background()

	// PASO 1: Obtener identidad real y token del usuario vía OAuth
	log.Comentario("INFO", "🔐 Paso 1: Autenticando usuario vía OAuth...")
	userEmail, token, err := ua.getUserEmailAndToken(ctx, log)
	if err != nil {
		return "", fmt.Errorf("error obteniendo identidad: %v", err)
	}
	log.Comentario("SUCCESS", fmt.Sprintf("✅ Usuario autenticado: %s", userEmail))

	// PASO 2: Usar el TOKEN DEL USUARIO para leer la hoja y validar permisos
	log.Comentario("INFO", "🔍 Paso 2: Validando permisos en hoja (usando tu token)...")
	if err := ua.validatePermissionsWithUserToken(ctx, userEmail, token, log); err != nil {
		return "", err
	}

	return userEmail, nil
}

func (ua *UserAuth) getUserEmailAndToken(ctx context.Context, log *Log) (string, *oauth2.Token, error) {
	token, err := ua.tokenFromFile(ua.config.TokenPath)
	if err == nil && token.Valid() {
		log.Comentario("INFO", "✅ Token de sesión encontrado.")
	} else {
		log.Comentario("INFO", "ℹ️ Iniciando autenticación en navegador...")
		token, err = ua.getTokenInteractively(ctx, log)
		if err != nil {
			return "", nil, err
		}
	}

	client := ua.oauthConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return "", nil, fmt.Errorf("error obteniendo información del usuario: %v", err)
	}
	defer resp.Body.Close()

	var userInfo struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return "", nil, fmt.Errorf("error decodificando información: %v", err)
	}

	return userInfo.Email, token, nil
}

func (ua *UserAuth) getTokenInteractively(ctx context.Context, log *Log) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("no se pudo iniciar servidor local: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	redirectURL := fmt.Sprintf("http://localhost:%d/", port)
	ua.oauthConfig.RedirectURL = redirectURL
	authURL := ua.oauthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	log.Comentario("INFO", "🌐 Abriendo navegador...")
	ua.openBrowser(authURL)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	srv := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port)}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no se recibió código")
			return
		}
		w.Write([]byte("✅ Autenticación exitosa. Cierra esta pestaña."))
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
	case err := <-errCh:
		srv.Shutdown(ctx)
		return nil, err
	case <-time.After(2 * time.Minute):
		srv.Shutdown(ctx)
		return nil, fmt.Errorf("tiempo de espera agotado")
	}

	token, err := ua.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("error intercambiando código: %v", err)
	}

	if err := ua.saveToken(ua.config.TokenPath, token); err != nil {
		log.Comentario("WARNING", fmt.Sprintf("⚠️ No se pudo guardar token: %v", err))
	}

	return token, nil
}

// validatePermissionsWithUserToken usa el TOKEN DEL USUARIO para leer la hoja
func (ua *UserAuth) validatePermissionsWithUserToken(ctx context.Context, userEmail string, token *oauth2.Token, log *Log) error {
	// Crear servicio de Sheets USANDO EL TOKEN DEL USUARIO (NO service account)
	srv, err := sheets.NewService(ctx, option.WithTokenSource(ua.oauthConfig.TokenSource(ctx, token)))
	if err != nil {
		return fmt.Errorf("error creando servicio de Sheets: %v", err)
	}

	// Leer hojas
	readSheet := func(rangeName string) ([][]interface{}, error) {
		resp, err := srv.Spreadsheets.Values.Get(ua.config.SpreadsheetID, rangeName).Do()
		if err != nil {
			return nil, err
		}
		return resp.Values, nil
	}

	usersData, err := readSheet("users!A:G")
	if err != nil {
		return fmt.Errorf("error leyendo hoja 'users' (¿Tienes acceso de Lector?): %v", err)
	}

	rolePermsData, err := readSheet("rol_x_permiso!A:B")
	if err != nil {
		return fmt.Errorf("error leyendo hoja 'rol_x_permiso': %v", err)
	}

	permsData, err := readSheet("permisos!A:C")
	if err != nil {
		return fmt.Errorf("error leyendo hoja 'permisos': %v", err)
	}

	// Construir mapas
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

	// Validar usuario
	user, exists := userMap[userEmail]
	if !exists {
		return fmt.Errorf("el correo %s no está registrado en la hoja 'users'", userEmail)
	}

	if user.Status != "activo" && user.Status != "active" {
		return fmt.Errorf("el usuario %s no está activo (estado: %s)", userEmail, user.Status)
	}

	// Validar permisos del rol
	permisosDelRol, exists := rolePermsMap[user.IDRol]
	if !exists {
		return fmt.Errorf("el rol %s no tiene permisos asignados", user.IDRol)
	}

	hasPermission := false
	for _, idPermiso := range permisosDelRol {
		if nombre, ok := permsMap[idPermiso]; ok {
			if strings.EqualFold(nombre, ua.config.RequiredPermission) {
				hasPermission = true
				break
			}
		}
	}

	if !hasPermission {
		return fmt.Errorf("el usuario %s (Rol: %s) no tiene el permiso: '%s'",
			userEmail, user.IDRol, ua.config.RequiredPermission)
	}

	log.Comentario("SUCCESS", fmt.Sprintf("✅ Usuario %s validado correctamente con permiso '%s'",
		userEmail, ua.config.RequiredPermission))
	return nil
}

func (ua *UserAuth) tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	token := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(token)
	return token, err
}

func (ua *UserAuth) saveToken(file string, token *oauth2.Token) error {
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

func (ua *UserAuth) openBrowser(url string) {
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
