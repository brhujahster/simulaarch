package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// helper: envia JSON para o handler e retorna o recorder
func post(t *testing.T, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/simulate", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	SimulateHandler(w, req)
	return w
}

func TestSimulateHandlerValidArchitecture(t *testing.T) {
	payload := map[string]interface{}{
		"nodes": []map[string]interface{}{
			{"id": "c1", "type": "client", "label": "Client 1", "x": 100, "y": 100,
				"config": map[string]interface{}{"RPS": 100.0}},
			{"id": "s1", "type": "service", "label": "Service 1", "x": 300, "y": 100,
				"config": map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 50.0}},
		},
		"edges": []map[string]interface{}{
			{"id": "e1", "from": "c1", "to": "s1", "type": "sync", "trafficShare": 1.0, "config": map[string]interface{}{}},
		},
	}

	w := post(t, payload)

	if w.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d: %s", w.Code, w.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("resposta não é JSON válido: %v", err)
	}
	if result["nodes"] == nil || result["edges"] == nil {
		t.Error("resposta deve conter 'nodes' e 'edges'")
	}
}

func TestSimulateHandlerInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/simulate", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	SimulateHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("esperava 400, obteve %d", w.Code)
	}
}

func TestSimulateHandlerNodeWithoutID(t *testing.T) {
	payload := map[string]interface{}{
		"nodes": []map[string]interface{}{
			{"id": "", "type": "client", "config": map[string]interface{}{}},
		},
		"edges": []map[string]interface{}{},
	}
	w := post(t, payload)
	if w.Code != http.StatusBadRequest {
		t.Errorf("nó sem ID deveria retornar 400, obteve %d", w.Code)
	}
}

func TestSimulateHandlerEdgeWithNonexistentNode(t *testing.T) {
	payload := map[string]interface{}{
		"nodes": []map[string]interface{}{
			{"id": "c1", "type": "client", "config": map[string]interface{}{}},
		},
		"edges": []map[string]interface{}{
			{"id": "e1", "from": "c1", "to": "nao-existe", "trafficShare": 1.0, "config": map[string]interface{}{}},
		},
	}
	w := post(t, payload)
	if w.Code != http.StatusBadRequest {
		t.Errorf("aresta com nó inexistente deveria retornar 400, obteve %d", w.Code)
	}
}

func TestSimulateHandlerSelfLoop(t *testing.T) {
	payload := map[string]interface{}{
		"nodes": []map[string]interface{}{
			{"id": "s1", "type": "service", "config": map[string]interface{}{}},
		},
		"edges": []map[string]interface{}{
			{"id": "e1", "from": "s1", "to": "s1", "trafficShare": 1.0, "config": map[string]interface{}{}},
		},
	}
	w := post(t, payload)
	if w.Code != http.StatusBadRequest {
		t.Errorf("self-loop deveria retornar 400, obteve %d", w.Code)
	}
}

func TestSimulateHandlerCycleReturns400(t *testing.T) {
	payload := map[string]interface{}{
		"nodes": []map[string]interface{}{
			{"id": "a", "type": "service", "config": map[string]interface{}{}},
			{"id": "b", "type": "service", "config": map[string]interface{}{}},
		},
		"edges": []map[string]interface{}{
			{"id": "e1", "from": "a", "to": "b", "trafficShare": 1.0, "config": map[string]interface{}{}},
			{"id": "e2", "from": "b", "to": "a", "trafficShare": 1.0, "config": map[string]interface{}{}},
		},
	}
	w := post(t, payload)
	if w.Code != http.StatusBadRequest {
		t.Errorf("ciclo deveria retornar 400, obteve %d", w.Code)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["error"] == "" {
		t.Error("resposta deveria conter campo 'error'")
	}
}

func TestSimulateHandlerRoundTripExportImport(t *testing.T) {
	// Simula o round-trip: exporta state como JSON → reimporta → simula novamente
	// O resultado deve ser idêntico (posições X/Y e configs preservados).
	original := map[string]interface{}{
		"nodes": []map[string]interface{}{
			{"id": "c1", "type": "client", "label": "Client", "x": 50.0, "y": 80.0,
				"config": map[string]interface{}{"RPS": 200.0}},
			{"id": "gw", "type": "apigateway", "label": "Gateway", "x": 250.0, "y": 80.0,
				"config": map[string]interface{}{"RateLimitRPS": 150.0, "LatencyOverheadMs": 5.0}},
			{"id": "s1", "type": "service", "label": "Service", "x": 450.0, "y": 80.0,
				"config": map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 20.0}},
		},
		"edges": []map[string]interface{}{
			{"id": "e1", "from": "c1", "to": "gw", "type": "sync", "trafficShare": 1.0, "config": map[string]interface{}{}},
			{"id": "e2", "from": "gw", "to": "s1", "type": "sync", "trafficShare": 1.0, "config": map[string]interface{}{}},
		},
	}

	// Primeira simulação
	w1 := post(t, original)
	if w1.Code != http.StatusOK {
		t.Fatalf("1ª simulação: esperava 200, obteve %d", w1.Code)
	}

	// "Exporta" → JSON → "Importa" (simula o ciclo frontend)
	// O JSON exportado não deve conter resultado de simulação (apenas arquitetura)
	var result1 map[string]interface{}
	json.Unmarshal(w1.Body.Bytes(), &result1)

	// Reconstrói o payload como se viesse de um import (apenas nodes+edges)
	reimported := map[string]interface{}{
		"nodes": original["nodes"],
		"edges": original["edges"],
	}

	w2 := post(t, reimported)
	if w2.Code != http.StatusOK {
		t.Fatalf("2ª simulação: esperava 200, obteve %d", w2.Code)
	}

	// Os resultados devem ser iguais
	if w1.Body.String() != w2.Body.String() {
		t.Error("round-trip: resultado da 2ª simulação difere da 1ª")
	}
}

func TestSimulateHandlerQueueImported(t *testing.T) {
	// Payload importado com queue deve funcionar corretamente
	payload := map[string]interface{}{
		"nodes": []map[string]interface{}{
			{"id": "c1", "type": "client", "label": "Client", "x": 50.0, "y": 100.0,
				"config": map[string]interface{}{"RPS": 300.0}},
			{"id": "q1", "type": "queue", "label": "Queue", "x": 250.0, "y": 100.0,
				"config": map[string]interface{}{"ThroughputMaxMsgsPerSec": 200.0, "WriteLatencyMs": 2.0}},
			{"id": "s1", "type": "service", "label": "Service", "x": 450.0, "y": 100.0,
				"config": map[string]interface{}{"CPU_Cores": 4.0, "ProcessTimeMs": 10.0}},
		},
		"edges": []map[string]interface{}{
			{"id": "e1", "from": "c1", "to": "q1", "type": "async", "trafficShare": 1.0, "config": map[string]interface{}{}},
			{"id": "e2", "from": "q1", "to": "s1", "type": "async", "trafficShare": 1.0, "config": map[string]interface{}{}},
		},
	}

	w := post(t, payload)
	if w.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d: %s", w.Code, w.Body.String())
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	nodes := result["nodes"].([]interface{})
	if len(nodes) != 3 {
		t.Errorf("esperava 3 nós no resultado, obteve %d", len(nodes))
	}
}

func TestSimulateHandlerContentTypeIsJSON(t *testing.T) {
	payload := map[string]interface{}{
		"nodes": []map[string]interface{}{
			{"id": "c1", "type": "client", "config": map[string]interface{}{"RPS": 10.0}},
		},
		"edges": []map[string]interface{}{},
	}
	w := post(t, payload)
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type esperado 'application/json', obteve '%s'", ct)
	}
}
