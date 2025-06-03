package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	calendar "google.golang.org/api/calendar/v3"
)

var (
	tmpl     = template.Must(template.ParseFiles("templates/index.html"))
	srv      *calendar.Service
	oauthCfg *oauth2.Config
)

func main() {
	ctx := context.Background()

	// Cargar credenciales
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("No se pudo leer el archivo de credenciales: %v", err)
	}

	// Configurar OAuth2
	oauthCfg, err = google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		log.Fatalf("No se pudo parsear el archivo de credenciales: %v", err)
	}

	// Obtener cliente autenticado
	client := getClient(ctx, oauthCfg)

	// Crear servicio de calendario
	srv, err = calendar.New(client)
	if err != nil {
		log.Fatalf("No se pudo crear el servicio de calendario: %v", err)
	}

	// Rutas
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/reservar", reservarHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	fmt.Println("Servidor iniciado en http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Handler para la página principal
func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		tmpl.Execute(w, nil)
	}
}

// Handler para reservar turno
func reservarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		nombre := r.FormValue("nombre")
		email := r.FormValue("email")
		fecha := r.FormValue("fecha")
		hora := r.FormValue("hora")

		// Parsear fecha y hora
		inicio, err := time.Parse("2006-01-02T15:04", fmt.Sprintf("%sT%s", fecha, hora))
		if err != nil {
			http.Error(w, "Fecha u hora inválida", http.StatusBadRequest)
			return
		}
		fin := inicio.Add(1 * time.Hour)

		// Crear evento
		evento := &calendar.Event{
			Summary:     fmt.Sprintf("Consulta con %s", nombre),
			Description: "Turno reservado desde la web",
			Start: &calendar.EventDateTime{
				DateTime: inicio.Format(time.RFC3339),
				TimeZone: "America/Argentina/Buenos_Aires",
			},
			End: &calendar.EventDateTime{
				DateTime: fin.Format(time.RFC3339),
				TimeZone: "America/Argentina/Buenos_Aires",
			},
			Attendees: []*calendar.EventAttendee{
				{Email: email},
			},
		}

		// Insertar evento en el calendario
		_, err = srv.Events.Insert("primary", evento).Do()
		if err != nil {
			http.Error(w, "Error al crear el evento", http.StatusInternalServerError)
			return
		}

		// Confirmación
		tmpl.Execute(w, struct {
			Confirmacion bool
			Nombre       string
			Fecha        string
			Hora         string
		}{
			Confirmacion: true,
			Nombre:       nombre,
			Fecha:        fecha,
			Hora:         hora,
		})
	}
}

// Obtener cliente autenticado
func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(ctx, tok)
}

// Obtener token desde la web
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Visita la siguiente URL para autorizar la aplicación:\n%v\n", authURL)

	var authCode string
	fmt.Print("Ingresa el código de autorización: ")
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("No se pudo leer el código de autorización: %v", err)
	}

	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		log.Fatalf("No se pudo obtener el token: %v", err)
	}
	return tok
}

// Leer token desde archivo
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Guardar token en archivo
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Guardando token en %s\n", path)
	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("No se pudo crear el archivo de token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
