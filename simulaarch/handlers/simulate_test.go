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

func TestSimulateHandlerGatewayWrongTypeBypasses(t *testing.T) {
	// Regressão: o frontend armazenava o gateway como type="gateway" (interno)
	// mas enviava esse valor cru ao backend, que espera "apigateway".
	// Com type="gateway" o motor cai no default e NÃO aplica rate limit.
	// O fix em buildSimulatePayload (toBackendType) converte antes de enviar.
	// Este teste documenta o comportamento do backend: "gateway" ≠ "apigateway".
	payload := map[string]interface{}{
		"nodes": []map[string]interface{}{
			{"id": "c1", "type": "client", "config": map[string]interface{}{"RPS": 500.0}},
			{"id": "gw", "type": "gateway", // tipo ERRADO — frontend não deveria enviar assim
				"config": map[string]interface{}{"RateLimitRPS": 100.0, "LatencyOverheadMs": 5.0}},
			{"id": "s1", "type": "service",
				"config": map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 20.0}},
		},
		"edges": []map[string]interface{}{
			{"id": "e1", "from": "c1", "to": "gw", "trafficShare": 1.0, "config": map[string]interface{}{}},
			{"id": "e2", "from": "gw", "to": "s1", "trafficShare": 1.0, "config": map[string]interface{}{}},
		},
	}
	w := post(t, payload)
	if w.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d", w.Code)
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	nodes := result["nodes"].([]interface{})
	var gwNode map[string]interface{}
	for _, n := range nodes {
		nm := n.(map[string]interface{})
		if nm["id"] == "gw" {
			gwNode = nm
		}
	}
	if gwNode == nil {
		t.Fatal("nó gw não encontrado")
	}
	// Com type="gateway" (errado), o motor usa default: sem rate limit.
	// effectiveRPS deve ser igual ao inRPS (500), não ao limite (100).
	eff := gwNode["effectiveRPS"].(float64)
	if eff != 500.0 {
		t.Errorf("com type='gateway' (errado) effectiveRPS esperado 500 (sem rate limit), obteve %.1f", eff)
	}
	// utilization deve ser 0 (nó não reconhecido, não calcula utilização)
	util := gwNode["utilization"].(float64)
	if util != 0.0 {
		t.Errorf("com type='gateway' (errado) utilization esperado 0, obteve %.2f", util)
	}
}

func TestSimulateHandlerGatewayDropsInResult(t *testing.T) {
	// Gateway com RateLimitRPS=50 recebendo 200 RPS → utilização=4 (CRITICAL)
	// A resposta deve conter o nó gateway com utilization > 1 para o dashboard calcular drops
	payload := map[string]interface{}{
		"nodes": []map[string]interface{}{
			{"id": "c1", "type": "client", "label": "Client", "x": 50.0, "y": 100.0,
				"config": map[string]interface{}{"RPS": 200.0}},
			{"id": "gw", "type": "apigateway", "label": "Gateway", "x": 250.0, "y": 100.0,
				"config": map[string]interface{}{"RateLimitRPS": 50.0, "LatencyOverheadMs": 2.0}},
			{"id": "s1", "type": "service", "label": "Service", "x": 450.0, "y": 100.0,
				"config": map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 20.0}},
		},
		"edges": []map[string]interface{}{
			{"id": "e1", "from": "c1", "to": "gw", "trafficShare": 1.0, "config": map[string]interface{}{}},
			{"id": "e2", "from": "gw", "to": "s1", "trafficShare": 1.0, "config": map[string]interface{}{}},
		},
	}
	w := post(t, payload)
	if w.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d: %s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)

	nodes := result["nodes"].([]interface{})
	var gwNode map[string]interface{}
	for _, n := range nodes {
		nm := n.(map[string]interface{})
		if nm["id"] == "gw" {
			gwNode = nm
		}
	}
	if gwNode == nil {
		t.Fatal("nó gateway não encontrado no resultado")
	}
	util := gwNode["utilization"].(float64)
	if util <= 1.0 {
		t.Errorf("gateway deveria ter utilization > 1 (saturado), obteve %.2f", util)
	}
	if gwNode["status"] != "CRITICAL" {
		t.Errorf("gateway saturado deveria ter status CRITICAL, obteve %s", gwNode["status"])
	}
	// effectiveRPS deve ser igual ao RateLimitRPS (50), não ao inRPS (200)
	eff := gwNode["effectiveRPS"].(float64)
	if eff != 50.0 {
		t.Errorf("gateway effectiveRPS esperado 50, obteve %.1f", eff)
	}
}

func TestSimulateHandlerQueueLagInMetrics(t *testing.T) {
	// Queue com throughput=100 recebendo 300 RPS → lagEst=200 msg/s
	payload := map[string]interface{}{
		"nodes": []map[string]interface{}{
			{"id": "c1", "type": "client", "label": "Client", "x": 50.0, "y": 100.0,
				"config": map[string]interface{}{"RPS": 300.0}},
			{"id": "q1", "type": "queue", "label": "Queue", "x": 250.0, "y": 100.0,
				"config": map[string]interface{}{"ThroughputMaxMsgsPerSec": 100.0, "WriteLatencyMs": 1.0}},
			{"id": "s1", "type": "service", "label": "Service", "x": 450.0, "y": 100.0,
				"config": map[string]interface{}{"CPU_Cores": 4.0, "ProcessTimeMs": 5.0}},
		},
		"edges": []map[string]interface{}{
			{"id": "e1", "from": "c1", "to": "q1", "trafficShare": 1.0, "config": map[string]interface{}{}},
			{"id": "e2", "from": "q1", "to": "s1", "trafficShare": 1.0, "config": map[string]interface{}{}},
		},
	}
	w := post(t, payload)
	if w.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d: %s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)

	nodes := result["nodes"].([]interface{})
	var qNode map[string]interface{}
	for _, n := range nodes {
		nm := n.(map[string]interface{})
		if nm["id"] == "q1" {
			qNode = nm
		}
	}
	if qNode == nil {
		t.Fatal("nó queue não encontrado no resultado")
	}
	metrics := qNode["metrics"].(map[string]interface{})
	lagEst := metrics["lagEst"].(float64)
	if lagEst != 200.0 {
		t.Errorf("lagEst esperado 200, obteve %.1f", lagEst)
	}
}

func TestSimulateHandlerMultiNodeFanOutTraffic(t *testing.T) {
	// Verifica que tráfego com fan-out (trafficShare) é distribuído corretamente
	// entre dois serviços — relevante para multi-select e visualização de arestas
	payload := map[string]interface{}{
		"nodes": []map[string]interface{}{
			{"id": "c1", "type": "client", "label": "Client", "x": 50.0, "y": 200.0,
				"config": map[string]interface{}{"RPS": 100.0}},
			{"id": "gw", "type": "apigateway", "label": "Gateway", "x": 250.0, "y": 200.0,
				"config": map[string]interface{}{"RateLimitRPS": 200.0, "LatencyOverheadMs": 5.0}},
			{"id": "sa", "type": "service", "label": "Service A", "x": 450.0, "y": 100.0,
				"config": map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 20.0}},
			{"id": "sb", "type": "service", "label": "Service B", "x": 450.0, "y": 300.0,
				"config": map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 20.0}},
		},
		"edges": []map[string]interface{}{
			{"id": "e1", "from": "c1", "to": "gw", "trafficShare": 1.0, "config": map[string]interface{}{}},
			{"id": "e2", "from": "gw", "to": "sa", "trafficShare": 0.5, "config": map[string]interface{}{}},
			{"id": "e3", "from": "gw", "to": "sb", "trafficShare": 0.5, "config": map[string]interface{}{}},
		},
	}
	w := post(t, payload)
	if w.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d: %s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)

	nodes := result["nodes"].([]interface{})
	nodeMap := map[string]map[string]interface{}{}
	for _, n := range nodes {
		nm := n.(map[string]interface{})
		nodeMap[nm["id"].(string)] = nm
	}

	// Service A e B devem receber 50 RPS cada (fan-out com trafficShare=0.5)
	saRPS := nodeMap["sa"]["effectiveRPS"].(float64)
	sbRPS := nodeMap["sb"]["effectiveRPS"].(float64)
	if saRPS != 50.0 {
		t.Errorf("Service A effectiveRPS esperado 50, obteve %.1f", saRPS)
	}
	if sbRPS != 50.0 {
		t.Errorf("Service B effectiveRPS esperado 50, obteve %.1f", sbRPS)
	}

	// Gateway não saturado (100 < 200 RPS)
	if nodeMap["gw"]["status"] != "OK" {
		t.Errorf("Gateway esperado OK, obteve %s", nodeMap["gw"]["status"])
	}

	// Resultado deve ter 2 rotas (Client→gw→sa e Client→gw→sb)
	routes := result["routes"].([]interface{})
	if len(routes) != 2 {
		t.Errorf("esperava 2 rotas (fan-out), obteve %d", len(routes))
	}
}

func TestSimulateHandlerIgnoresExtraFieldsLikeName(t *testing.T) {
	// O campo "name" do cenário (4.5) é salvo no JSON exportado mas não enviado ao /simulate.
	// Este teste garante que um payload com campo extra não causa erro 400.
	payload := map[string]interface{}{
		"name": "Cenário de Teste",
		"nodes": []map[string]interface{}{
			{"id": "c1", "type": "client", "label": "Client", "x": 50.0, "y": 100.0,
				"config": map[string]interface{}{"RPS": 50.0}},
			{"id": "s1", "type": "service", "label": "Service", "x": 250.0, "y": 100.0,
				"config": map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 20.0}},
		},
		"edges": []map[string]interface{}{
			{"id": "e1", "from": "c1", "to": "s1", "trafficShare": 1.0, "config": map[string]interface{}{}},
		},
	}
	w := post(t, payload)
	if w.Code != http.StatusOK {
		t.Fatalf("payload com campo 'name' deveria retornar 200, obteve %d: %s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["nodes"] == nil {
		t.Error("resposta deve conter 'nodes'")
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
