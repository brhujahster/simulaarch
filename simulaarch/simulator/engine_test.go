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

// ─── Testes específicos de K8s Cluster (task 3.2) ─────────────────────────────

func TestClusterNodeAloneIsAccepted(t *testing.T) {
	// Cluster isolado no canvas não deve causar erro
	arch := models.Architecture{
		Nodes: []models.Node{
			node("k1", "cluster", map[string]interface{}{
				"MinReplicas":   1.0,
				"MaxReplicas":   5.0,
				"HPA_Threshold": 0.7,
			}),
		},
		Edges: []models.Edge{},
	}
	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("cluster isolado não deveria retornar erro: %v", err)
	}
	k, ok := findNode(res, "k1")
	if !ok {
		t.Fatal("k1 não encontrado no resultado")
	}
	if k.Status != models.StatusOK {
		t.Errorf("cluster sem carga deveria ser OK, obteve %s", k.Status)
	}
}

func TestClusterWithAssociatedServiceInPayload(t *testing.T) {
	// Service com clusterRef: HPA deve escalar para absorver a carga.
	// MaxRPS/replica=(1000*2)/25=80. RPS=80. HPA=0.7. MinReplicas=2. MaxReplicas=10.
	// singleUtil=1.0 → needed=ceil(1.0/0.7)=2 → actualReplicas=max(2,2)=2
	// effectiveMaxRPS=160 → util=0.5 → OK
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 80.0}),
			node("k1", "cluster", map[string]interface{}{
				"MinReplicas":   2.0,
				"MaxReplicas":   10.0,
				"HPA_Threshold": 0.7,
			}),
			{
				ID:    "s1",
				Type:  "service",
				Label: "s1",
				Config: map[string]interface{}{
					"CPU_Cores":     2.0,
					"ProcessTimeMs": 25.0,
					"clusterRef":    "k1",
				},
			},
		},
		Edges: []models.Edge{
			edge("e1", "c1", "s1", 1.0),
		},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("service com clusterRef não deveria causar erro: %v", err)
	}

	s1, _ := findNode(res, "s1")
	if s1.Status != models.StatusOK {
		t.Errorf("esperava OK (HPA escalou para 2 réplicas), obteve %s", s1.Status)
	}
	if s1.EffectiveRPS != 80 {
		t.Errorf("effectiveRPS esperado 80, obteve %.1f", s1.EffectiveRPS)
	}
	if replicas := s1.Metrics["replicas"].(float64); replicas != 2 {
		t.Errorf("replicas esperado 2, obteve %.0f", replicas)
	}
}

func TestClusterGatewayServiceChain(t *testing.T) {
	// Client → Gateway → Service (no cluster) → resultado correto
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 150.0}),
			node("gw", "apigateway", map[string]interface{}{"RateLimitRPS": 100.0, "LatencyOverheadMs": 3.0}),
			node("k1", "cluster", map[string]interface{}{"MinReplicas": 2.0, "MaxReplicas": 8.0, "HPA_Threshold": 0.6}),
			{
				ID:    "s1",
				Type:  "service",
				Label: "s1",
				Config: map[string]interface{}{
					"CPU_Cores":     4.0,
					"ProcessTimeMs": 20.0,
					"clusterRef":    "k1",
				},
			},
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
	if gw.EffectiveRPS != 100 {
		t.Errorf("gateway deve limitar a 100 RPS, obteve %.1f", gw.EffectiveRPS)
	}

	s1, _ := findNode(res, "s1")
	// MaxRPS = (1000*4)/20 = 200 → util = 100/200 = 0.5 → OK
	if s1.Status != models.StatusOK {
		t.Errorf("service esperado OK (util=0.5), obteve %s", s1.Status)
	}
	// latência: gateway(3ms) + service(20ms) = 23ms
	if s1.LatencyMs != 23 {
		t.Errorf("latencyMs esperado 23, obteve %.1f", s1.LatencyMs)
	}
}

func TestMultipleClustersIndependent(t *testing.T) {
	// Dois clusters independentes não interferem entre si; HPA escala cada um.
	// sa: MaxRPS/rep=40, RPS=50, HPA=0.7 → needed=ceil(1.25/0.7)=2 → util=50/80=0.625 → OK
	// sb: MaxRPS/rep=100, RPS=200, HPA=0.6 → needed=ceil(2.0/0.6)=4 → util=200/400=0.5 → OK
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 50.0}),
			node("c2", "client", map[string]interface{}{"RPS": 200.0}),
			node("k1", "cluster", map[string]interface{}{"MinReplicas": 1.0, "MaxReplicas": 4.0, "HPA_Threshold": 0.7}),
			node("k2", "cluster", map[string]interface{}{"MinReplicas": 2.0, "MaxReplicas": 10.0, "HPA_Threshold": 0.6}),
			{ID: "sa", Type: "service", Label: "sa",
				Config: map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 50.0, "clusterRef": "k1"}},
			{ID: "sb", Type: "service", Label: "sb",
				Config: map[string]interface{}{"CPU_Cores": 1.0, "ProcessTimeMs": 10.0, "clusterRef": "k2"}},
		},
		Edges: []models.Edge{
			edge("e1", "c1", "sa", 1.0),
			edge("e2", "c2", "sb", 1.0),
		},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	sa, _ := findNode(res, "sa")
	sb, _ := findNode(res, "sb")

	if sa.Status != models.StatusOK {
		t.Errorf("sa: HPA escalou para 2 réplicas, esperava OK, obteve %s", sa.Status)
	}
	if saRep := sa.Metrics["replicas"].(float64); saRep != 2 {
		t.Errorf("sa: esperava 2 réplicas, obteve %.0f", saRep)
	}

	if sb.Status != models.StatusOK {
		t.Errorf("sb: HPA escalou para 4 réplicas, esperava OK, obteve %s", sb.Status)
	}
	if sbRep := sb.Metrics["replicas"].(float64); sbRep != 4 {
		t.Errorf("sb: esperava 4 réplicas, obteve %.0f", sbRep)
	}
}

// ─── Testes de Queue assíncrona e K8s HPA (task 3.3) ─────────────────────────

func TestQueueRateLimit(t *testing.T) {
	// Queue com throughput menor que o ingress deve limitar egress e gerar lag
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 800.0}),
			node("q1", "queue", map[string]interface{}{
				"ThroughputMaxMsgsPerSec": 500.0,
				"WriteLatencyMs":          3.0,
			}),
		},
		Edges: []models.Edge{edge("e1", "c1", "q1", 1.0)},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	q, _ := findNode(res, "q1")
	// egressRPS = min(800, 500) = 500
	if q.EffectiveRPS != 500 {
		t.Errorf("egressRPS esperado 500, obteve %.1f", q.EffectiveRPS)
	}
	// utilização = 800/500 = 1.6 → CRITICAL
	if q.Status != models.StatusCritical {
		t.Errorf("queue deveria ser CRITICAL, obteve %s", q.Status)
	}
	// metrics deve conter lagEst = 800-500 = 300
	if q.Metrics == nil {
		t.Fatal("queue deve ter metrics")
	}
	if lag, ok := q.Metrics["lagEst"].(float64); !ok || lag != 300 {
		t.Errorf("lagEst esperado 300, obteve %v", q.Metrics["lagEst"])
	}
	if ing, ok := q.Metrics["ingressRPS"].(float64); !ok || ing != 800 {
		t.Errorf("ingressRPS esperado 800, obteve %v", q.Metrics["ingressRPS"])
	}
}

func TestQueueUnderCapacityNoLag(t *testing.T) {
	// Queue com ingress < throughput: egressRPS = ingressRPS, lagEst = 0
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 200.0}),
			node("q1", "queue", map[string]interface{}{
				"ThroughputMaxMsgsPerSec": 1000.0,
				"WriteLatencyMs":          2.0,
			}),
		},
		Edges: []models.Edge{edge("e1", "c1", "q1", 1.0)},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	q, _ := findNode(res, "q1")
	if q.EffectiveRPS != 200 {
		t.Errorf("egressRPS esperado 200, obteve %.1f", q.EffectiveRPS)
	}
	if q.Status != models.StatusOK {
		t.Errorf("queue 20%% utilização deveria ser OK, obteve %s", q.Status)
	}
	if lag := q.Metrics["lagEst"].(float64); lag != 0 {
		t.Errorf("lagEst esperado 0, obteve %.1f", lag)
	}
}

func TestQueueAsyncLatencyBarrier(t *testing.T) {
	// A latência downstream de uma queue deve ser independente da upstream.
	// Client(upstream pesado) → Queue(writeLatency=5ms) → Service(10ms)
	// Latência do service deve ser 5+10=15ms, NÃO acumular latência do client.
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 50.0}),
			node("gw", "apigateway", map[string]interface{}{
				"RateLimitRPS":      1000.0,
				"LatencyOverheadMs": 50.0, // latência pesada upstream
			}),
			node("q1", "queue", map[string]interface{}{
				"ThroughputMaxMsgsPerSec": 1000.0,
				"WriteLatencyMs":          5.0,
			}),
			node("s1", "service", map[string]interface{}{
				"CPU_Cores":     2.0,
				"ProcessTimeMs": 10.0,
			}),
		},
		Edges: []models.Edge{
			edge("e1", "c1", "gw", 1.0),
			edge("e2", "gw", "q1", 1.0),
			edge("e3", "q1", "s1", 1.0),
		},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	q, _ := findNode(res, "q1")
	// Queue deve ter nodeLatency = writeLatencyMs = 5ms (barreira)
	if q.LatencyMs != 5 {
		t.Errorf("queue latencyMs esperado 5 (barreira async), obteve %.1f", q.LatencyMs)
	}

	s1, _ := findNode(res, "s1")
	// Latência do service = queue(5ms) + processTime(10ms) = 15ms
	// Sem a barreira seria: client(0) + gateway(50ms) + queue(5ms) + service(10ms) = 65ms
	if s1.LatencyMs != 15 {
		t.Errorf("service latencyMs esperado 15 (barreira async), obteve %.1f", s1.LatencyMs)
	}
}

func TestQueueMultipleProducers(t *testing.T) {
	// Dois clients enviam para a mesma queue: IngressRPS = soma
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 300.0}),
			node("c2", "client", map[string]interface{}{"RPS": 200.0}),
			node("q1", "queue", map[string]interface{}{
				"ThroughputMaxMsgsPerSec": 400.0,
				"WriteLatencyMs":          1.0,
			}),
		},
		Edges: []models.Edge{
			edge("e1", "c1", "q1", 1.0),
			edge("e2", "c2", "q1", 1.0),
		},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	q, _ := findNode(res, "q1")
	// ingressRPS = 300+200 = 500, egressRPS = min(500,400) = 400
	if q.EffectiveRPS != 400 {
		t.Errorf("egressRPS esperado 400, obteve %.1f", q.EffectiveRPS)
	}
	if ing := q.Metrics["ingressRPS"].(float64); ing != 500 {
		t.Errorf("ingressRPS esperado 500, obteve %.1f", ing)
	}
	if lag := q.Metrics["lagEst"].(float64); lag != 100 {
		t.Errorf("lagEst esperado 100, obteve %.1f", lag)
	}
	// utilização = 500/400 = 1.25 → CRITICAL
	if q.Status != models.StatusCritical {
		t.Errorf("queue deveria ser CRITICAL, obteve %s", q.Status)
	}
}

func TestK8sHPAScaleUp(t *testing.T) {
	// Service com 1 réplica seria CRITICAL, HPA deve escalar para atender
	// MaxRPS/réplica = (1000*1)/10 = 100. RPS = 250. HPA_Threshold = 0.7.
	// needed = ceil((250/100) / 0.7) = ceil(2.5/0.7) = ceil(3.57) = 4 réplicas
	// MaxReplicas = 5 → actualReplicas = 4
	// effectiveMaxRPS = 100 * 4 = 400 → util = 250/400 = 0.625 → OK
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 250.0}),
			node("k1", "cluster", map[string]interface{}{
				"MinReplicas":   1.0,
				"MaxReplicas":   5.0,
				"HPA_Threshold": 0.7,
			}),
			{
				ID:    "s1",
				Type:  "service",
				Label: "s1",
				Config: map[string]interface{}{
					"CPU_Cores":     1.0,
					"ProcessTimeMs": 10.0,
					"clusterRef":    "k1",
				},
			},
		},
		Edges: []models.Edge{edge("e1", "c1", "s1", 1.0)},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	s1, _ := findNode(res, "s1")
	if s1.Status != models.StatusOK {
		t.Errorf("service com HPA deveria ser OK após scale-up, obteve %s", s1.Status)
	}
	if s1.Metrics == nil {
		t.Fatal("service deve ter metrics de cluster")
	}
	if replicas := s1.Metrics["replicas"].(float64); replicas != 4 {
		t.Errorf("replicas esperado 4, obteve %.0f", replicas)
	}
	if isSat := s1.Metrics["isSaturated"].(bool); isSat {
		t.Error("isSaturated deve ser false (4 réplicas são suficientes)")
	}
}

func TestK8sHPAMinReplicas(t *testing.T) {
	// Tráfego baixo: HPA não deve cair abaixo do MinReplicas
	// MaxRPS/réplica = 100. RPS = 10. HPA=0.7 → needed=ceil(0.1/0.7)=1 = MinReplicas
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 10.0}),
			node("k1", "cluster", map[string]interface{}{
				"MinReplicas":   2.0,
				"MaxReplicas":   10.0,
				"HPA_Threshold": 0.7,
			}),
			{
				ID:    "s1",
				Type:  "service",
				Label: "s1",
				Config: map[string]interface{}{
					"CPU_Cores":     1.0,
					"ProcessTimeMs": 10.0,
					"clusterRef":    "k1",
				},
			},
		},
		Edges: []models.Edge{edge("e1", "c1", "s1", 1.0)},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	s1, _ := findNode(res, "s1")
	// needed = ceil((10/100)/0.7) = ceil(0.143) = 1, mas MinReplicas = 2
	if replicas := s1.Metrics["replicas"].(float64); replicas != 2 {
		t.Errorf("replicas esperado 2 (MinReplicas), obteve %.0f", replicas)
	}
	if s1.Status != models.StatusOK {
		t.Errorf("service com baixo tráfego deveria ser OK, obteve %s", s1.Status)
	}
}

func TestK8sHPASaturationAtMaxReplicas(t *testing.T) {
	// Tráfego tão alto que mesmo MaxReplicas não é suficiente
	// MaxRPS/réplica = 100. RPS = 600. MaxReplicas = 4.
	// needed = ceil((600/100)/0.7) = ceil(8.57) = 9, mas maxR=4 → actualReplicas=4
	// effectiveMaxRPS = 400 → util = 600/400 = 1.5 → CRITICAL + isSaturated=true
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 600.0}),
			node("k1", "cluster", map[string]interface{}{
				"MinReplicas":   1.0,
				"MaxReplicas":   4.0,
				"HPA_Threshold": 0.7,
			}),
			{
				ID:    "s1",
				Type:  "service",
				Label: "s1",
				Config: map[string]interface{}{
					"CPU_Cores":     1.0,
					"ProcessTimeMs": 10.0,
					"clusterRef":    "k1",
				},
			},
		},
		Edges: []models.Edge{edge("e1", "c1", "s1", 1.0)},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	s1, _ := findNode(res, "s1")
	if s1.Status != models.StatusCritical {
		t.Errorf("service saturado deveria ser CRITICAL, obteve %s", s1.Status)
	}
	if replicas := s1.Metrics["replicas"].(float64); replicas != 4 {
		t.Errorf("replicas esperado 4 (MaxReplicas), obteve %.0f", replicas)
	}
	if isSat := s1.Metrics["isSaturated"].(bool); !isSat {
		t.Error("isSaturated deve ser true (600 > 100*4=400)")
	}
}

func TestMixedSyncAsyncPaths(t *testing.T) {
	// Arquitetura com caminho síncrono e assíncrono em paralelo:
	// Client → (sync) Service A
	// Client → (async via Queue) Service B
	// Ambos devem ser calculados corretamente e independentemente.
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 100.0}),
			node("sa", "service", map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 20.0}),
			node("q1", "queue", map[string]interface{}{
				"ThroughputMaxMsgsPerSec": 1000.0,
				"WriteLatencyMs":          3.0,
			}),
			node("sb", "service", map[string]interface{}{"CPU_Cores": 1.0, "ProcessTimeMs": 15.0}),
		},
		Edges: []models.Edge{
			edge("e1", "c1", "sa", 0.6),
			edge("e2", "c1", "q1", 0.4),
			edge("e3", "q1", "sb", 1.0),
		},
	}

	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	sa, _ := findNode(res, "sa")
	// sa recebe 60 RPS; MaxRPS=(1000*2)/20=100 → util=0.6 → OK; latência=20ms
	if sa.Status != models.StatusOK {
		t.Errorf("sa esperado OK, obteve %s", sa.Status)
	}
	if sa.LatencyMs != 20 {
		t.Errorf("sa latencyMs esperado 20, obteve %.1f", sa.LatencyMs)
	}

	q, _ := findNode(res, "q1")
	// queue recebe 40 RPS → OK; latência=3ms (barreira)
	if q.LatencyMs != 3 {
		t.Errorf("queue latencyMs esperado 3, obteve %.1f", q.LatencyMs)
	}

	sb, _ := findNode(res, "sb")
	// sb recebe 40 RPS; MaxRPS=(1000*1)/15≈66.7 → util≈0.6 → OK
	// latência = queue(3ms) + processTime(15ms) = 18ms (independente do upstream)
	if sb.LatencyMs != 18 {
		t.Errorf("sb latencyMs esperado 18 (3+15), obteve %.1f", sb.LatencyMs)
	}
	if sb.Status != models.StatusOK {
		t.Errorf("sb esperado OK, obteve %s", sb.Status)
	}
}

// ─── Testes de findRoutes (métricas de latência por rota) ─────────────────────

func TestRoutesSimplePath(t *testing.T) {
	// Client → Service: uma única rota, latência = processTimeMs
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 100.0}),
			node("s1", "service", map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 30.0}),
		},
		Edges: []models.Edge{edge("e1", "c1", "s1", 1.0)},
	}
	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(res.Routes) != 1 {
		t.Fatalf("esperava 1 rota, obteve %d", len(res.Routes))
	}
	r := res.Routes[0]
	// latência = edge(0) + service(30) = 30ms
	if r.LatencyMs != 30 {
		t.Errorf("latencyMs esperado 30, obteve %.1f", r.LatencyMs)
	}
	if r.P95Ms != 45 {
		t.Errorf("p95Ms esperado 45 (30×1.5), obteve %.1f", r.P95Ms)
	}
	if r.P99Ms != 60 {
		t.Errorf("p99Ms esperado 60 (30×2.0), obteve %.1f", r.P99Ms)
	}
	if len(r.Path) != 2 || r.Path[0] != "c1" || r.Path[1] != "s1" {
		t.Errorf("path inesperado: %v", r.Path)
	}
}

func TestRoutesWithAPIGateway(t *testing.T) {
	// Client → APIGateway(5ms) → Service(20ms): latência = 5 + 20 = 25ms
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 100.0}),
			node("gw", "apigateway", map[string]interface{}{"LatencyOverheadMs": 5.0, "RateLimitRPS": 500.0}),
			node("s1", "service", map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 20.0}),
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
	if len(res.Routes) != 1 {
		t.Fatalf("esperava 1 rota, obteve %d", len(res.Routes))
	}
	if res.Routes[0].LatencyMs != 25 {
		t.Errorf("latencyMs esperado 25 (5+20), obteve %.1f", res.Routes[0].LatencyMs)
	}
}

func TestRoutesFanOut(t *testing.T) {
	// Client → Service → [ServiceA, ServiceB]: duas rotas distintas
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 100.0}),
			node("s1", "service", map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 10.0}),
			node("sa", "service", map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 20.0}),
			node("sb", "service", map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 30.0}),
		},
		Edges: []models.Edge{
			edge("e1", "c1", "s1", 1.0),
			edge("e2", "s1", "sa", 0.5),
			edge("e3", "s1", "sb", 0.5),
		},
	}
	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(res.Routes) != 2 {
		t.Fatalf("esperava 2 rotas (fan-out), obteve %d", len(res.Routes))
	}
	// latências: c1→s1→sa = 10+20=30ms; c1→s1→sb = 10+30=40ms
	latencies := map[float64]bool{}
	for _, r := range res.Routes {
		latencies[r.LatencyMs] = true
	}
	if !latencies[30] {
		t.Error("esperava rota com latência 30ms (c1→s1→sa)")
	}
	if !latencies[40] {
		t.Error("esperava rota com latência 40ms (c1→s1→sb)")
	}
}

func TestRoutesQueueAsAsyncBarrier(t *testing.T) {
	// Client → Queue(writeLatency=5ms) → Service(25ms)
	// Queue reseta acumulador: latência = 5 + 25 = 30ms (não inclui upstream do client)
	arch := models.Architecture{
		Nodes: []models.Node{
			node("c1", "client", map[string]interface{}{"RPS": 100.0}),
			node("q1", "queue", map[string]interface{}{"ThroughputMaxMsgsPerSec": 200.0, "WriteLatencyMs": 5.0}),
			node("s1", "service", map[string]interface{}{"CPU_Cores": 2.0, "ProcessTimeMs": 25.0}),
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
	if len(res.Routes) != 1 {
		t.Fatalf("esperava 1 rota, obteve %d", len(res.Routes))
	}
	// queue reinicia acumulador; latência do caminho = writeLatency(5) + processTime(25)
	if res.Routes[0].LatencyMs != 30 {
		t.Errorf("latencyMs esperado 30 (barreira assíncrona: 5+25), obteve %.1f", res.Routes[0].LatencyMs)
	}
}

func TestRoutesNoClientNoRoutes(t *testing.T) {
	// Sem nó client: nenhuma rota deve ser enumerada
	arch := models.Architecture{
		Nodes: []models.Node{
			node("s1", "service", map[string]interface{}{"CPU_Cores": 1.0, "ProcessTimeMs": 10.0}),
			node("s2", "service", map[string]interface{}{"CPU_Cores": 1.0, "ProcessTimeMs": 20.0}),
		},
		Edges: []models.Edge{edge("e1", "s1", "s2", 1.0)},
	}
	res, err := Simulate(arch)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(res.Routes) != 0 {
		t.Errorf("esperava 0 rotas sem client, obteve %d", len(res.Routes))
	}
}
