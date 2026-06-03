package controllers

import (
	"net/http"

	"github.com/specialkapa/matrigonio/internal/server"
)

type HomeController struct {
	*server.APIConfig
}

func (c *HomeController) HandlerHome(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if err := c.Templates.ExecuteTemplate(
		w,
		"index.html",
		map[string]any{"appname": c.AppName},
	); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
}
