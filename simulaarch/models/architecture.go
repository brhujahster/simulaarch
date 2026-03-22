package models

type Architecture struct {
    Nodes []Node `json:"nodes"`
    Edges []Edge `json:"edges"`
}

type Node struct {
    ID     string                 `json:"id"`
    Type   string                 `json:"type"`
    Label  string                 `json:"label"`
    X      float64                `json:"x"`
    Y      float64                `json:"y"`
    Config map[string]interface{} `json:"config"`
}

type Edge struct {
    ID           string                 `json:"id"`
    From         string                 `json:"from"`
    To           string                 `json:"to"`
    Type         string                 `json:"type"`
    TrafficShare float64                `json:"trafficShare"`
    Config       map[string]interface{} `json:"config"`
}