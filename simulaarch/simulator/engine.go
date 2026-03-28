package simulator

import (
	"fmt"
	"math"

	"simulaarch/models"
)

type graph struct {
	nodes    map[string]models.Node
	outEdges map[string][]models.Edge
}

func buildGraph(arch models.Architecture) graph {
	g := graph{
		nodes:    make(map[string]models.Node, len(arch.Nodes)),
		outEdges: make(map[string][]models.Edge),
	}
	for _, n := range arch.Nodes {
		g.nodes[n.ID] = n
	}
	for _, e := range arch.Edges {
		g.outEdges[e.From] = append(g.outEdges[e.From], e)
	}
	return g
}

// detectCycles usa DFS com coloração (white=0, gray=1, black=2).
func detectCycles(g graph) error {
	color := make(map[string]int)
	var dfs func(id string) bool
	dfs = func(id string) bool {
		color[id] = 1
		for _, e := range g.outEdges[id] {
			if color[e.To] == 1 {
				return true
			}
			if color[e.To] == 0 {
				if dfs(e.To) {
					return true
				}
			}
		}
		color[id] = 2
		return false
	}
	for id := range g.nodes {
		if color[id] == 0 {
			if dfs(id) {
				return fmt.Errorf("ciclo detectado no grafo a partir do nó '%s'", id)
			}
		}
	}
	return nil
}

func getFloat(config map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := config[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		}
	}
	return defaultVal
}

// findRoutes enumera todas as rotas distintas (Client → nó folha) no grafo
// e calcula p50, p95 e p99 de latência por aproximação simples.
// Nós Queue atuam como barreiras assíncronas: resetam o acumulador de latência.
func findRoutes(g graph) []models.RouteResult {
	var routes []models.RouteResult

	var dfs func(path []string, accLatency float64)
	dfs = func(path []string, accLatency float64) {
		id := path[len(path)-1]
		n := g.nodes[id]

		var contrib float64
		switch n.Type {
		case "apigateway":
			contrib = getFloat(n.Config, "LatencyOverheadMs", 0)
		case "service":
			contrib = getFloat(n.Config, "ProcessTimeMs", 0)
		case "queue":
			contrib = getFloat(n.Config, "WriteLatencyMs", 0)
			accLatency = 0 // barreira assíncrona: reseta cadeia de latência
		}

		pathLatency := accLatency + contrib

		outEdges := g.outEdges[id]
		if len(outEdges) == 0 {
			// nó folha: registra rota
			routes = append(routes, models.RouteResult{
				Path:      append([]string{}, path...),
				LatencyMs: pathLatency,
				P95Ms:     pathLatency * 1.5,
				P99Ms:     pathLatency * 2.0,
			})
			return
		}

		for _, e := range outEdges {
			edgeLatMs := getFloat(e.Config, "LatencyMs", 0)
			newPath := make([]string, len(path)+1)
			copy(newPath, path)
			newPath[len(path)] = e.To
			dfs(newPath, pathLatency+edgeLatMs)
		}
	}

	for id, n := range g.nodes {
		if n.Type == "client" {
			dfs([]string{id}, 0)
		}
	}

	return routes
}

func statusFor(utilization float64) string {
	if utilization >= 1.0 {
		return models.StatusCritical
	}
	if utilization >= 0.8 {
		return models.StatusAlert
	}
	return models.StatusOK
}

// Simulate executa a simulação de carga sobre a arquitetura fornecida,
// suportando caminhos síncronos e assíncronos (Queue como barreira de fluxo).
func Simulate(arch models.Architecture) (models.SimulationResult, error) {
	result := models.SimulationResult{
		Nodes: []models.NodeResult{},
		Edges: []models.EdgeResult{},
	}

	if len(arch.Nodes) == 0 {
		return result, nil
	}

	g := buildGraph(arch)

	if err := detectCycles(g); err != nil {
		return result, err
	}

	// rpsAt[nodeID]     = RPS acumulado recebido pelo nó
	// latencyAt[nodeID] = latência acumulada até esse nó (caminho mais longo)
	rpsAt := make(map[string]float64)
	latencyAt := make(map[string]float64)
	edgeRPS := make(map[string]float64)
	edgeLatency := make(map[string]float64)

	for _, n := range arch.Nodes {
		if n.Type == "client" {
			rpsAt[n.ID] = getFloat(n.Config, "RPS", 0)
		}
	}

	// Ordenação topológica (Kahn)
	inDegree := make(map[string]int)
	for id := range g.nodes {
		inDegree[id] = 0
	}
	for _, edges := range g.outEdges {
		for _, e := range edges {
			inDegree[e.To]++
		}
	}
	topoQueue := []string{}
	for id, deg := range inDegree {
		if deg == 0 {
			topoQueue = append(topoQueue, id)
		}
	}
	processed := []string{}
	for len(topoQueue) > 0 {
		cur := topoQueue[0]
		topoQueue = topoQueue[1:]
		processed = append(processed, cur)
		for _, e := range g.outEdges[cur] {
			inDegree[e.To]--
			if inDegree[e.To] == 0 {
				topoQueue = append(topoQueue, e.To)
			}
		}
	}

	for _, id := range processed {
		n := g.nodes[id]
		inRPS := rpsAt[id]
		inLatency := latencyAt[id]

		var effectiveRPS, nodeLatency, utilization float64
		var nodeMetrics map[string]interface{}

		switch n.Type {

		case "client":
			effectiveRPS = getFloat(n.Config, "RPS", 0)
			nodeLatency = 0
			utilization = 0

		case "apigateway":
			rateLimitRPS := getFloat(n.Config, "RateLimitRPS", math.MaxFloat64)
			latencyOverhead := getFloat(n.Config, "LatencyOverheadMs", 0)
			effectiveRPS = math.Min(inRPS, rateLimitRPS)
			nodeLatency = inLatency + latencyOverhead
			if rateLimitRPS > 0 {
				utilization = inRPS / rateLimitRPS
			}

		case "queue":
			throughputMax := getFloat(n.Config, "ThroughputMaxMsgsPerSec", math.MaxFloat64)
			writeLatencyMs := getFloat(n.Config, "WriteLatencyMs", 0)

			egressRPS := math.Min(inRPS, throughputMax)
			lagEst := math.Max(0, inRPS-throughputMax)

			if throughputMax > 0 {
				utilization = inRPS / throughputMax
			}
			effectiveRPS = egressRPS
			// Barreira assíncrona: a latência do caminho downstream começa
			// a partir do tempo de escrita na fila, independente do upstream.
			nodeLatency = writeLatencyMs

			nodeMetrics = map[string]interface{}{
				"ingressRPS": inRPS,
				"egressRPS":  egressRPS,
				"lagEst":     lagEst,
			}

		case "service":
			cpuCores := getFloat(n.Config, "CPU_Cores", 1)
			processTimeMs := getFloat(n.Config, "ProcessTimeMs", 0)

			maxRPSPerReplica := 0.0
			if processTimeMs > 0 {
				maxRPSPerReplica = (1000 * cpuCores) / processTimeMs
			}

			actualReplicas := 1.0
			if clusterID, ok := n.Config["clusterRef"].(string); ok && clusterID != "" {
				if cl, exists := g.nodes[clusterID]; exists && cl.Type == "cluster" {
					minR := getFloat(cl.Config, "MinReplicas", 1)
					maxR := getFloat(cl.Config, "MaxReplicas", 1)
					hpa := getFloat(cl.Config, "HPA_Threshold", 0.7)

					if hpa > 0 && maxRPSPerReplica > 0 {
						singleUtil := inRPS / maxRPSPerReplica
						needed := math.Ceil(singleUtil / hpa)
						actualReplicas = math.Max(minR, math.Min(maxR, needed))
					} else {
						actualReplicas = minR
					}

					isSaturated := inRPS > maxRPSPerReplica*maxR
					nodeMetrics = map[string]interface{}{
						"replicas":    actualReplicas,
						"isSaturated": isSaturated,
					}
				}
			}

			effectiveMaxRPS := maxRPSPerReplica * actualReplicas
			effectiveRPS = inRPS
			if effectiveMaxRPS > 0 {
				utilization = inRPS / effectiveMaxRPS
			}
			nodeLatency = inLatency + processTimeMs

		case "cluster":
			// Cluster é metadata para serviços associados; não transporta tráfego.
			effectiveRPS = 0
			nodeLatency = 0
			utilization = 0

		default:
			effectiveRPS = inRPS
			nodeLatency = inLatency
		}

		result.Nodes = append(result.Nodes, models.NodeResult{
			ID:           id,
			Status:       statusFor(utilization),
			Utilization:  utilization,
			EffectiveRPS: effectiveRPS,
			LatencyMs:    nodeLatency,
			Metrics:      nodeMetrics,
		})

		for _, e := range g.outEdges[id] {
			share := e.TrafficShare
			if share <= 0 {
				share = 1.0
			}
			flowRPS := effectiveRPS * share
			edgeLatMs := getFloat(e.Config, "LatencyMs", 0)

			edgeRPS[e.ID] += flowRPS
			edgeLatency[e.ID] = edgeLatMs

			rpsAt[e.To] += flowRPS
			candidate := nodeLatency + edgeLatMs
			if candidate > latencyAt[e.To] {
				latencyAt[e.To] = candidate
			}
		}
	}

	for _, e := range arch.Edges {
		result.Edges = append(result.Edges, models.EdgeResult{
			ID:        e.ID,
			RPSFlow:   edgeRPS[e.ID],
			LatencyMs: edgeLatency[e.ID],
		})
	}

	result.Routes = findRoutes(g)

	return result, nil
}
