package main

import (
	"log"
	"net/http"

	"simulaarch/handlers"
)

// noCacheFileServer wraps http.FileServer adicionando Cache-Control: no-cache
// para garantir que o browser sempre valide arquivos estáticos no servidor,
// evitando que versões antigas de app.js ou style.css sejam exibidas.
func noCacheFileServer(fs http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		fs.ServeHTTP(w, r)
	})
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", handlers.IndexHandler)
	mux.HandleFunc("POST /simulate", handlers.SimulateHandler)
	mux.Handle("GET /static/", noCacheFileServer(
		http.StripPrefix("/static/", http.FileServer(http.Dir("static"))),
	))

	log.Println("SimulaArch rodando em http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}
