package simulator

import (
    "simulaarch/models"
)

func Simulate(arch models.Architecture) (models.SimulationResult, error) {
    result := models.SimulationResult{
        Nodes: []models.NodeResult{},
        Edges: []models.EdgeResult{},
    }

    for _, node := range arch.Nodes {
        result.Nodes = append(result.Nodes, models.NodeResult{
            ID:     node.ID,
            Status: models.StatusOK,
        })
    }

    return result, nil
}