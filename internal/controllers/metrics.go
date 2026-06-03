package controllers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/specialkapa/matrigonio/internal/server"
)

type MetricsController struct {
	*server.APIConfig
}

func (c *MetricsController) HandlerMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	data, _ := json.Marshal(struct {
		Hits int32 `json:"hits"`
	}{
		Hits: (*c).HomePageHits.Load(),
	})
	_, _ = w.Write(data)
}

// TODO: figure out how to prevent anyone from hitting this endpoint without requiring auth
func (c *MetricsController) HandlerReset(w http.ResponseWriter, r *http.Request) {
	if c.Platform != "dev" {
		responseWithError(w, errors.New("unauthorized access to reset endpoint"), "Unauthorized", 403)
		return
	}
	(*c).HomePageHits.Store(0)

	w.WriteHeader(200)
	_, _ = w.Write([]byte("Hits reset to 0 and users purged"))
}

func (c *MetricsController) HandlerResetHomeCookie(w http.ResponseWriter, r *http.Request) {
	if c.Platform != "dev" {
		responseWithError(w, errors.New("unauthorized access to reset endpoint"), "Unauthorized", 403)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "home_seen",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Platform == "prod",
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("home_seen cookie cleared"))
}
