package simulator

import (
	"fmt"
	"math"

	"simulaarch/models"
)

// nodeMap e edgeMap para acesso rápido por ID
type graph struct {
	nodes    map[string]models.Node
	outEdges map[string][]models.Edge // nodeID -> arestas de saída
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

// detectCycles usa DFS com coloração (white=0, gray=1, black=2)
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

// Simulate executa a simulação de carga síncrona sobre a arquitetura fornecida.
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

	// rpsAt[nodeID] = RPS acumulado recebido pelo nó
	rpsAt := make(map[string]float64)
	// latencyAt[nodeID] = latência acumulada até esse nó (caminho mais longo até ele)
	latencyAt := make(map[string]float64)
	// edgeRPS[edgeID] = RPS efetivo fluindo na aresta
	edgeRPS := make(map[string]float64)
	// edgeLatency[edgeID] = latência da aresta
	edgeLatency := make(map[string]float64)

	// Inicializa nós client com seus RPS configurados
	for _, n := range arch.Nodes {
		if n.Type == "client" {
			rps := getFloat(n.Config, "RPS", 0)
			rpsAt[n.ID] = rps
		}
	}

	// Ordenação topológica (Kahn's algorithm) para processar nós em ordem
	inDegree := make(map[string]int)
	for id := range g.nodes {
		inDegree[id] = 0
	}
	for _, edges := range g.outEdges {
		for _, e := range edges {
			inDegree[e.To]++
		}
	}

	queue := []string{}
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	processed := []string{}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		processed = append(processed, cur)
		for _, e := range g.outEdges[cur] {
			inDegree[e.To]--
			if inDegree[e.To] == 0 {
				queue = append(queue, e.To)
			}
		}
	}

	// Processa cada nó em ordem topológica
	for _, id := range processed {
		n := g.nodes[id]
		inRPS := rpsAt[id]
		inLatency := latencyAt[id]

		var effectiveRPS float64
		var nodeLatency float64
		var utilization float64
		status := models.StatusOK

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

		case "service":
			cpuCores := getFloat(n.Config, "CPU_Cores", 1)
			processTimeMs := getFloat(n.Config, "ProcessTimeMs", 0)

			maxRPS := 0.0
			if processTimeMs > 0 {
				maxRPS = (1000 * cpuCores) / processTimeMs
			}
			effectiveRPS = inRPS
			if maxRPS > 0 {
				utilization = inRPS / maxRPS
			}
			nodeLatency = inLatency + processTimeMs

		default:
			effectiveRPS = inRPS
			nodeLatency = inLatency
		}

		if utilization >= 1.0 {
			status = models.StatusCritical
		} else if utilization >= 0.8 {
			status = models.StatusAlert
		}

		result.Nodes = append(result.Nodes, models.NodeResult{
			ID:           id,
			Status:       status,
			Utilization:  utilization,
			EffectiveRPS: effectiveRPS,
			LatencyMs:    nodeLatency,
		})

		// Propaga RPS para arestas de saída respeitando TrafficShare
		for _, e := range g.outEdges[id] {
			share := e.TrafficShare
			if share <= 0 {
				share = 1.0
			}
			flowRPS := effectiveRPS * share
			edgeLatMs := getFloat(e.Config, "LatencyMs", 0)

			edgeRPS[e.ID] += flowRPS
			edgeLatency[e.ID] = edgeLatMs

			// Acumula RPS e latência no nó destino
			rpsAt[e.To] += flowRPS
			// Latência do destino = máximo dos caminhos chegando até ele
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

	return result, nil
}
