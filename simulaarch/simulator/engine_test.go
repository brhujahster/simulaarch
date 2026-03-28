package simulator

import (
	"testing"

	"simulaarch/models"
)

// helpers

func node(id, typ string, config map[string]interface{}) models.Node {
	return models.Node{ID: id, Type: typ, Label: id, Config: config}
}

func edge(id, from, to string, share float64) models.Edge {
	return models.Edge{ID: id, From: from, To: to, TrafficShare: share, Config: map[string]interface{}{}}
}

func findNode(res models.SimulationResult, id string) (models.NodeResult, bool) {
	for _, n := range res.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return models.NodeResult{}, false
}

func findEdge(res models.SimulationResult, id string) (models.EdgeResult, bool) {
	for _, e := range res.Edges {
		if e.ID == id {
			return e, true
		}
	}
	return models.EdgeResult{}, false
}

// ─── Testes ───────────────────────────────────────────────────────────────────

func TestEmptyArchitecture(t *testing.T) {
	res, err := Simulate(models.Architecture{})
	if err != nil {
		t.Fatalf("esperava sem erro, obteve: %v", err)
	}
	if len(res.Nodes) != 0 || len(res.Edges) != 0 {
		t.Errorf("resultado deveria estar vazio para arquitetura vazia")
	}
}

func TestClientServiceSimple(t *testing.T) {
	// Client 100 RPS → Service (2 cores, 50ms) → MaxRPS=40, utilização=2.5
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 100.0}),
			node("s1", "service", map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 50.0}),
		},
		Edges: []models.Edge{edge("e1", "c1", "s1", 1.0)},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	s, ok := findNode(res, "s1")
	if !ok {
		t.Fatal("s1 não encontrado no resultado")
	}
	if s.Status != models.StatusCritical {
		t.Errorf("esperava CRITICAL, obteve %s", s.Status)
	}
	if s.EffectiveRPS != 100 {
		t.Errorf("effectiveRPS esperado 100, obteve %.1f", s.EffectiveRPS)
	}
	if s.LatencyMs != 50 {
		t.Errorf("latencyMs esperado 50, obteve %.1f", s.LatencyMs)
	}

	e1, _ := findEdge(res, "e1")
	if e1.RPSFlow != 100 {
		t.Errorf("rpsFlow esperado 100, obteve %.1f", e1.RPSFlow)
	}
}

func TestServiceUtilizationThresholds(t *testing.T) {
	cases := []struct {
		rps            float64
		expectedStatus string
	}{
		{30, models.StatusOK},      // 30/40 = 0.75 < 0.8
		{35, models.StatusAlert},   // 35/40 = 0.875 >= 0.8
		{45, models.StatusCritical},// 45/40 = 1.125 >= 1.0
	}
	// MaxRPS = (1000 * 2) / 50 = 40
	for _, tc := range cases {
		arch := models.Architecture{
			Nodes: []models.Node{
				node("c1", "client", map[string]interface{}{"RPS": tc.rps}),
				node("s1", "service", map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 50.0}),
			},
			Edges: []models.Edge{edge("e1", "c1", "s1", 1.0)},
		}
		res, err := Simulate(arch)
		if err != nil {
			t.Fatalf("rps=%.0f: erro inesperado: %v", tc.rps, err)
		}
		s, _ := findNode(res, "s1")
		if s.Status != tc.expectedStatus {
			t.Errorf("rps=%.0f: esperava %s, obteve %s", tc.rps, tc.expectedStatus, s.Status)
		}
	}
}

func TestAPIGatewayRateLimit(t *testing.T) {
	// Client 500 RPS → Gateway (limit 200, overhead 5ms) → Service (1 core, 10ms)
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 500.0}),
			node("gw", "apigateway", map[string]interface{}{"RateLimitRPS": 200.0, "LatencyOverheadMs": 5.0}),
			node("s1", "service", map[string]interface{}{"CPU_Cores": 1.0, "ProcessTimeMs": 10.0}),
		},
		Edges: []models.Edge{
			edge("e1", "c1", "gw", 1.0),
			edge("e2", "gw", "s1", 1.0),
		},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	gw, _ := findNode(res, "gw")
	if gw.EffectiveRPS != 200 {
		t.Errorf("gateway effectiveRPS esperado 200 (rate limit), obteve %.1f", gw.EffectiveRPS)
	}
	if gw.LatencyMs != 5 {
		t.Errorf("gateway latencyMs esperado 5, obteve %.1f", gw.LatencyMs)
	}
	if gw.Status != models.StatusCritical {
		t.Errorf("gateway utilização=2.5 deveria ser CRITICAL, obteve %s", gw.Status)
	}

	s1, _ := findNode(res, "s1")
	if s1.EffectiveRPS != 200 {
		t.Errorf("service effectiveRPS esperado 200, obteve %.1f", s1.EffectiveRPS)
	}
	// latência: gateway(5ms) + service(10ms) = 15ms
	if s1.LatencyMs != 15 {
		t.Errorf("service latencyMs esperado 15, obteve %.1f", s1.LatencyMs)
	}
}

func TestTrafficShareFanOut(t *testing.T) {
	// Client 100 RPS → 60% Service A, 40% Service B
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 100.0}),
			node("sa", "service", map[string]interface{}{"CPU_Cores": 1.0, "ProcessTimeMs": 10.0}),
			node("sb", "service", map[string]interface{}{"CPU_Cores": 1.0, "ProcessTimeMs": 10.0}),
		},
		Edges: []models.Edge{
			edge("e1", "c1", "sa", 0.6),
			edge("e2", "c1", "sb", 0.4),
		},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	sa, _ := findNode(res, "sa")
	sb, _ := findNode(res, "sb")

	if sa.EffectiveRPS != 60 {
		t.Errorf("sa esperado 60 RPS, obteve %.1f", sa.EffectiveRPS)
	}
	if sb.EffectiveRPS != 40 {
		t.Errorf("sb esperado 40 RPS, obteve %.1f", sb.EffectiveRPS)
	}
}

func TestQueueNodePassthrough(t *testing.T) {
	// Client → Queue: queue deve receber o RPS e repassar sem crash
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 200.0}),
			node("q1", "queue", map[string]interface{}{"ThroughputMaxMsgsPerSec": 1000.0, "WriteLatencyMs": 2.0}),
		},
		Edges: []models.Edge{edge("e1", "c1", "q1", 1.0)},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	q, ok := findNode(res, "q1")
	if !ok {
		t.Fatal("q1 não encontrado no resultado")
	}
	// Sem lógica de queue ainda (task 3.3), effectiveRPS = RPS recebido
	if q.EffectiveRPS != 200 {
		t.Errorf("queue effectiveRPS esperado 200, obteve %.1f", q.EffectiveRPS)
	}

	e1, _ := findEdge(res, "e1")
	if e1.RPSFlow != 200 {
		t.Errorf("rpsFlow esperado 200, obteve %.1f", e1.RPSFlow)
	}
}

func TestQueueToService(t *testing.T) {
	// Client → Queue → Service: fluxo completo passando por queue
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 300.0}),
			node("q1", "queue", map[string]interface{}{"ThroughputMaxMsgsPerSec": 1000.0, "WriteLatencyMs": 2.0}),
			node("s1", "service", map[string]interface{}{"CPU_Cores": 4.0, "ProcessTimeMs": 20.0}),
		},
		Edges: []models.Edge{
			edge("e1", "c1", "q1", 1.0),
			edge("e2", "q1", "s1", 1.0),
		},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	s1, _ := findNode(res, "s1")
	if s1.EffectiveRPS != 300 {
		t.Errorf("service effectiveRPS esperado 300, obteve %.1f", s1.EffectiveRPS)
	}
	// MaxRPS = (1000 * 4) / 20 = 200 → util = 300/200 = 1.5 → CRITICAL
	if s1.Status != models.StatusCritical {
		t.Errorf("service deveria ser CRITICAL, obteve %s", s1.Status)
	}
}

func TestCycleDetection(t *testing.T) {
	arch := models.Architecture{
		Nodes: []models.Node{
			node("a", "service", map[string]interface{}{}),
			node("b", "service", map[string]interface{}{}),
		},
		Edges: []models.Edge{
			edge("e1", "a", "b", 1.0),
			edge("e2", "b", "a", 1.0),
		},
	}

	_, err := Simulate(arch)
	if err == nil {
		t.Error("esperava erro de ciclo, não obteve nenhum")
	}
}

func TestMultipleClientsConverging(t *testing.T) {
	// Dois clients somam RPS no service
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 60.0}),
			node("c2", "client", map[string]interface{}{"RPS": 40.0}),
			node("s1", "service", map[string]interface{}{"CPU_Cores": 1.0, "ProcessTimeMs": 10.0}),
		},
		Edges: []models.Edge{
			edge("e1", "c1", "s1", 1.0),
			edge("e2", "c2", "s1", 1.0),
		},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	s1, _ := findNode(res, "s1")
	// MaxRPS = 100, recebe 100 → util = 1.0 → CRITICAL
	if s1.EffectiveRPS != 100 {
		t.Errorf("service effectiveRPS esperado 100, obteve %.1f", s1.EffectiveRPS)
	}
	if s1.Status != models.StatusCritical {
		t.Errorf("esperava CRITICAL, obteve %s", s1.Status)
	}
}

func TestEdgeLatencyAccumulation(t *testing.T) {
	// Latência da aresta deve acumular no nó destino
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 10.0}),
			node("s1", "service", map[string]interface{}{"CPU_Cores": 4.0, "ProcessTimeMs": 30.0}),
		},
		Edges: []models.Edge{
			{ID: "e1", From: "c1", To: "s1", TrafficShare: 1.0, Config: map[string]interface{}{"LatencyMs": 20.0}},
		},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	s1, _ := findNode(res, "s1")
	// latência: aresta(20ms) + processTime(30ms) = 50ms
	if s1.LatencyMs != 50 {
		t.Errorf("latencyMs esperado 50 (20 aresta + 30 process), obteve %.1f", s1.LatencyMs)
	}

	e1, _ := findEdge(res, "e1")
	if e1.LatencyMs != 20 {
		t.Errorf("edge latencyMs esperado 20, obteve %.1f", e1.LatencyMs)
	}
}
