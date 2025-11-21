package scheduler

import (
	"github.com/sanchey92/go-cube/internal/node"
	"github.com/sanchey92/go-cube/internal/services/task"
)

type RoundRobin struct {
	Name       string
	LastWorker int
}

func (r *RoundRobin) SelectCandidates(t *task.Task, nodes []*node.Node) []*node.Node {
	return nodes
}

func (r *RoundRobin) Score(t *task.Task, nodes []*node.Node) map[string]float64 {
	nodesScore := make(map[string]float64, len(nodes))
	var newWorker int

	if r.LastWorker+1 < len(nodes) {
		newWorker = r.LastWorker + 1
	} else {
		newWorker = 0
		r.LastWorker = 0
	}

	for i, n := range nodes {
		if i == newWorker {
			nodesScore[n.Name] = 0.1
		} else {
			nodesScore[n.Name] = 1.0
		}
	}

	return nodesScore
}

func (r *RoundRobin) Pick(scores map[string]float64, candidates []*node.Node) *node.Node {
	var bestNode *node.Node
	var lovestScore float64

	for i, n := range candidates {
		if i == 0 {
			bestNode = n
			lovestScore = scores[n.Name]
			continue
		}
		if scores[n.Name] < lovestScore {
			lovestScore = scores[n.Name]
		}
	}

	return bestNode
}
