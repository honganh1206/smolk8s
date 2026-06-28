package scheduler

import (
	"github.com/honganh1206/smolk8s/internal/node"
	"github.com/honganh1206/smolk8s/internal/task"
)

type RoundRobin struct {
	Name     string
	LastNode int
}

func (r *RoundRobin) SelectCandidateNodes(t *task.Task, nodes []*node.Node) []*node.Node {
	return nodes
}

func (r *RoundRobin) Score(t *task.Task, nodes []*node.Node) map[string]float64 {
	nodeScores := make(map[string]float64)

	var newNode int
	if r.LastNode+1 < len(nodes) {
		// Next node in line is selected
		newNode = r.LastNode + 1
		r.LastNode++
	} else {
		// Wrap around , start from 1st node
		newNode = 0
		r.LastNode = 0
	}

	// Simple scoring method
	for idx, node := range nodes {
		if idx == newNode {
			nodeScores[node.Name] = 0.1
		} else {
			nodeScores[node.Name] = 1.0
		}
	}

	return nodeScores
}

func (r *RoundRobin) Pick(scores map[string]float64, candidates []*node.Node) *node.Node {
	var bestNode *node.Node
	var lowestScore float64

	for idx, node := range candidates {
		if idx == 0 {
			bestNode = node
			lowestScore = scores[node.Name]
			continue
		}

		// The best node is the one with the lowest score
		if scores[node.Name] < lowestScore {
			bestNode = node
			lowestScore = scores[node.Name]
		}
	}

	return bestNode
}
