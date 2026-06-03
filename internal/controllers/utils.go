package controllers

import (
	"encoding/json"
	"log"
	"net/http"
)

func responseWithError(w http.ResponseWriter, e error, m string, code int) {
	log.Print(e.Error())
	w.WriteHeader(code)
	data, _ := json.Marshal(struct {
		Error string `json:"error"`
	}{
		Error: m,
	})
	_, _ = w.Write(data)
}
