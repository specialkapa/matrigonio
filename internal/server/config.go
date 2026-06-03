package server

import (
	"html/template"
	"net/http"
	"sync/atomic"
)

type APIConfig struct {
	AppName      string
	Platform     string
	StaticDir    string
	Templates    *template.Template
	HomePageHits atomic.Int32
	Guests       *GuestStore
}

func (c *APIConfig) MiddlewareCountFirstHomeVisit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("home_seen"); err == http.ErrNoCookie {
			c.HomePageHits.Add(1)

			http.SetCookie(w, &http.Cookie{
				Name:     "home_seen",
				Value:    "1",
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Secure:   c.Platform == "prod",
			})
		}

		next.ServeHTTP(w, r)
	})
}
