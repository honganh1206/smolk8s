package scheduler

import (
	"github.com/honganh1206/smolk8s/internal/node"
	"github.com/honganh1206/smolk8s/internal/task"
)

type Scheduler interface {
	// SelectCandidateNodes filters out a list of nodes that meet resource requirements
	SelectCandidateNodes(t *task.Task, nodes []*node.Node) []*node.Node
	// Score scores each node based on a scheduling algorithm
	Score(t *task.Task, nodes []*node.Node) map[string]float64
	// Pick returns the node with the best scores
	Pick(scores map[string]float64, candidates []*node.Node) *node.Node
}
