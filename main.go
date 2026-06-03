package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/joho/godotenv"
	"github.com/specialkapa/matrigonio/internal/controllers"
	"github.com/specialkapa/matrigonio/internal/server"
)

func main() {
	// .env is optional: it's used in local dev but absent in most hosting
	// environments, so a missing file must not be fatal.
	_ = godotenv.Load()

	const (
		staticDir  = "./web/static/"
		guestsPath = "./internal/data/guests.csv"
	)

	// Hosts like Render/Fly/Cloud Run inject the port to listen on via $PORT.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	guests, err := server.LoadGuests(guestsPath)
	if err != nil {
		log.Printf("warning: could not load guest list from %s: %v", guestsPath, err)
	}

	c := server.APIConfig{
		AppName:      "Matrigonio",
		Platform:     os.Getenv("PLATFORM"),
		StaticDir:    staticDir,
		Templates:    template.Must(template.ParseGlob("./internal/templates/*")),
		HomePageHits: atomic.Int32{},
		Guests:       guests,
	}
	mc := controllers.MetricsController{&c}
	hc := controllers.HomeController{&c}
	menuC := controllers.MenuController{&c}

	mux := http.NewServeMux()
	fileServer := http.FileServer(http.Dir(staticDir))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
	mux.Handle("/app/", c.MiddlewareCountFirstHomeVisit(http.HandlerFunc(hc.HandlerHome)))
	mux.HandleFunc("GET /api/checkhealth", controllers.HandlerCheckHealth)
	mux.HandleFunc("GET /api/metrics", mc.HandlerMetrics)
	mux.HandleFunc("POST /api/reset", mc.HandlerReset)
	mux.HandleFunc("POST /api/reset-home-cookie", mc.HandlerResetHomeCookie)
	mux.HandleFunc("POST /api/menu-lookup", menuC.HandlerMenuLookup)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Serving static files from %s on port: %s\n", staticDir, port)
	log.Fatal(server.ListenAndServe())
}
