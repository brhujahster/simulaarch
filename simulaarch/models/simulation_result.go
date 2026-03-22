package models

const (
    StatusOK       = "OK"
    StatusAlert    = "ALERT"
    StatusCritical = "CRITICAL"
)

type SimulationResult struct {
    Nodes  []NodeResult  `json:"nodes"`
    Edges  []EdgeResult  `json:"edges"`
    Errors []string      `json:"errors,omitempty"`
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