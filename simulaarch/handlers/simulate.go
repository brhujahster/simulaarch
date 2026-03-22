package handlers

import (
    "encoding/json"
    "net/http"

    "simulaarch/models"
    "simulaarch/simulator"
)

func SimulateHandler(w http.ResponseWriter, r *http.Request) {
    var arch models.Architecture

    if err := json.NewDecoder(r.Body).Decode(&arch); err != nil {
        http.Error(w, `{"error":"JSON inválido"}`, http.StatusBadRequest)
        return
    }

    result, err := simulator.Simulate(arch)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}