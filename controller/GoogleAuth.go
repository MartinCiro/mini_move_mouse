package controller

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// ============================================================
// VALORES POR DEFECTO (Hardcodeados por el Administrador)
// ============================================================
const (
	DefaultSpreadsheetID      = "18lifTzZUTmIgDXWEwH8XeGx1_yXi3HwgJP1_l25Wb4Y"
	DefaultRequiredPermission = "ejx:mm"
	DefaultTokenPath          = "token.json"
)

// AuthConfig configuración para OAuth + Gviz
type AuthConfig struct {
	Enabled            *bool  `json:"enabled,omitempty"`
	SpreadsheetID      string `json:"spreadsheet_id,omitempty"`
	RequiredPermission string `json:"required_permission,omitempty"`
	TokenPath          string `json:"token_path,omitempty"`
	// ❌ Ya NO existe CredentialsPath
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

	// ✅ USAR CREDENCIALES EMBEBIDAS de credentials.go
	oauthConfig, err := google.ConfigFromJSON([]byte(OAuthCredentials), "https://www.googleapis.com/auth/userinfo.email")
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

	// PASO 1: Obtener identidad real vía OAuth
	log.Comentario("INFO", "🔐 Paso 1: Verificando identidad del usuario vía OAuth...")
	userEmail, err := ua.getUserEmail(ctx, log)
	if err != nil {
		return "", fmt.Errorf("error obteniendo identidad: %v", err)
	}
	log.Comentario("SUCCESS", fmt.Sprintf("✅ Identidad verificada: %s", userEmail))

	// PASO 2: Validar permisos leyendo la hoja pública vía Gviz (CSV)
	log.Comentario("INFO", "🔍 Paso 2: Validando permisos en hoja (modo público)...")
	if err := ua.validatePermissionsViaGviz(userEmail, log); err != nil {
		return "", err
	}

	return userEmail, nil
}

func (ua *UserAuth) getUserEmail(ctx context.Context, log *Log) (string, error) {
	token, err := ua.tokenFromFile(ua.config.TokenPath)
	if err == nil && token.Valid() {
		log.Comentario("INFO", "✅ Token de sesión encontrado. Usando identidad guardada.")
	} else {
		log.Comentario("INFO", "ℹ️ Iniciando flujo de autenticación en navegador...")
		token, err = ua.getTokenInteractively(ctx, log)
		if err != nil {
			return "", err
		}
	}

	client := ua.oauthConfig.Client(ctx, token)
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

	log.Comentario("INFO", "🌐 Abriendo navegador para autenticación...")
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

// validatePermissionsViaGviz lee las hojas como CSV público
func (ua *UserAuth) validatePermissionsViaGviz(userEmail string, log *Log) error {
	fetchCSV := func(sheetName string) ([][]string, error) {
		gvizURL := fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/gviz/tq?tqx=out:csv&sheet=%s",
			ua.config.SpreadsheetID, url.QueryEscape(sheetName))

		resp, err := http.Get(gvizURL)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("error HTTP %d al leer hoja %s", resp.StatusCode, sheetName)
		}

		reader := csv.NewReader(resp.Body)
		reader.LazyQuotes = true // Maneja mejor las comillas de Google

		var records [][]string
		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			// Limpiar comillas dobles que Google a veces añade
			for i := range record {
				record[i] = strings.Trim(record[i], "\"")
			}
			records = append(records, record)
		}
		return records, nil
	}

	usersData, err := fetchCSV("users")
	if err != nil {
		return fmt.Errorf("error leyendo hoja 'users': %v", err)
	}

	rolePermsData, err := fetchCSV("rol_x_permiso")
	if err != nil {
		return fmt.Errorf("error leyendo hoja 'rol_x_permiso': %v", err)
	}

	permsData, err := fetchCSV("permisos")
	if err != nil {
		return fmt.Errorf("error leyendo hoja 'permisos': %v", err)
	}

	// Construir mapas (lógica idéntica a la anterior)
	userMap := make(map[string]struct{ IDRol, Status string })
	for i, row := range usersData {
		if i == 0 || len(row) < 4 {
			continue
		}
		email := strings.TrimSpace(row[1])
		idRol := strings.TrimSpace(row[2])
		status := strings.ToLower(strings.TrimSpace(row[3]))
		if email != "" {
			userMap[email] = struct{ IDRol, Status string }{IDRol: idRol, Status: status}
		}
	}

	rolePermsMap := make(map[string][]string)
	for i, row := range rolePermsData {
		if i == 0 || len(row) < 2 {
			continue
		}
		idRol := strings.TrimSpace(row[0])
		idPermiso := strings.TrimSpace(row[1])
		if idRol != "" && idPermiso != "" {
			rolePermsMap[idRol] = append(rolePermsMap[idRol], idPermiso)
		}
	}

	permsMap := make(map[string]string)
	for i, row := range permsData {
		if i == 0 || len(row) < 2 {
			continue
		}
		idPermiso := strings.TrimSpace(row[0])
		nombrePermiso := strings.TrimSpace(row[1])
		if idPermiso != "" {
			permsMap[idPermiso] = nombrePermiso
		}
	}

	// Validar
	user, exists := userMap[userEmail]
	if !exists {
		return fmt.Errorf("el correo %s no está registrado en la hoja 'users'", userEmail)
	}
	if user.Status != "activo" && user.Status != "1" {
		return fmt.Errorf("el usuario %s no está activo (estado: %s)", userEmail, user.Status)
	}

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
		return fmt.Errorf("el usuario %s (Rol: %s) no tiene el permiso: '%s'", userEmail, user.IDRol, ua.config.RequiredPermission)
	}

	log.Comentario("SUCCESS", fmt.Sprintf("✅ Usuario %s validado correctamente con permiso '%s'", userEmail, ua.config.RequiredPermission))
	return nil
}

// --- Funciones auxiliares ---
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
