package handlers

import (
    "html/template"
    "net/http"
)

var tmpl = template.Must(template.ParseFiles("templates/index.html"))

func IndexHandler(w http.ResponseWriter, r *http.Request) {
    if err := tmpl.Execute(w, nil); err != nil {
        http.Error(w, "Erro ao renderizar template", http.StatusInternalServerError)
    }
}