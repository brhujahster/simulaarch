package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"simulaarch/models"
	"simulaarch/simulator"
)

func validateArchitecture(arch models.Architecture) error {
	nodeIDs := make(map[string]bool, len(arch.Nodes))

	for i, n := range arch.Nodes {
		if n.ID == "" {
			return fmt.Errorf("nó na posição %d não possui ID", i)
		}
		if nodeIDs[n.ID] {
			return fmt.Errorf("ID de nó duplicado: '%s'", n.ID)
		}
		nodeIDs[n.ID] = true
	}

	edgeIDs := make(map[string]bool, len(arch.Edges))
	for i, e := range arch.Edges {
		if e.ID == "" {
			return fmt.Errorf("aresta na posição %d não possui ID", i)
		}
		if edgeIDs[e.ID] {
			return fmt.Errorf("ID de aresta duplicado: '%s'", e.ID)
		}
		edgeIDs[e.ID] = true

		if e.From == "" {
			return fmt.Errorf("aresta '%s' não possui nó de origem (from)", e.ID)
		}
		if e.To == "" {
			return fmt.Errorf("aresta '%s' não possui nó de destino (to)", e.ID)
		}
		if !nodeIDs[e.From] {
			return fmt.Errorf("aresta '%s' referencia nó de origem inexistente: '%s'", e.ID, e.From)
		}
		if !nodeIDs[e.To] {
			return fmt.Errorf("aresta '%s' referencia nó de destino inexistente: '%s'", e.ID, e.To)
		}
		if e.From == e.To {
			return fmt.Errorf("aresta '%s' conecta um nó a ele mesmo: '%s'", e.ID, e.From)
		}
	}

	return nil
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func SimulateHandler(w http.ResponseWriter, r *http.Request) {
	var arch models.Architecture

	if err := json.NewDecoder(r.Body).Decode(&arch); err != nil {
		writeJSONError(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}

	if err := validateArchitecture(arch); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := simulator.Simulate(arch)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
