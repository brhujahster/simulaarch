package models

const (
    StatusOK       = "OK"
    StatusAlert    = "ALERT"
    StatusCritical = "CRITICAL"
)

type SimulationResult struct {
	Nodes  []NodeResult  `json:"nodes"`
	Edges  []EdgeResult  `json:"edges"`
	Routes []RouteResult `json:"routes,omitempty"`
	Errors []string      `json:"errors,omitempty"`
}

// RouteResult representa uma rota distinta do grafo (Client → folha)
// com métricas de latência estimadas por aproximação estatística simples.
type RouteResult struct {
	Path      []string `json:"path"`      // IDs dos nós em ordem
	LatencyMs float64  `json:"latencyMs"` // p50 / média estimada
	P95Ms     float64  `json:"p95Ms"`     // p50 × 1.5
	P99Ms     float64  `json:"p99Ms"`     // p50 × 2.0
}

type NodeResult struct {
    ID           string                 `json:"id"`
    Status       string                 `json:"status"`
    Utilization  float64                `json:"utilization"`
    EffectiveRPS float64                `json:"effectiveRPS"`
    LatencyMs    float64                `json:"latencyMs"`
    Metrics      map[string]interface{} `json:"metrics,omitempty"`
}

type EdgeResult struct {
    ID        string  `json:"id"`
    RPSFlow   float64 `json:"rpsFlow"`
    LatencyMs float64 `json:"latencyMs"`
}