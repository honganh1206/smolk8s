package scheduler

import (
	"testing"

	"github.com/honganh1206/smolk8s/internal/node"
	"github.com/honganh1206/smolk8s/internal/task"
)

func testNodes(names ...string) []*node.Node {
	nodes := make([]*node.Node, len(names))
	for i, n := range names {
		nodes[i] = &node.Node{Name: n}
	}
	return nodes
}

func TestSelectCandidateNodesReturnsAll(t *testing.T) {
	r := &RoundRobin{Name: "rr"}
	nodes := testNodes("n1", "n2", "n3")

	got := r.SelectCandidateNodes(&task.Task{}, nodes)

	// TODO: This has yet to test resource requirements
	if len(got) != len(nodes) {
		t.Fatalf("expected %d nodes, got %d", len(nodes), len(got))
	}
}

func TestScoreFavorsNextNode(t *testing.T) {
	r := &RoundRobin{Name: "rr"}
	nodes := testNodes("n1", "n2", "n3")

	// First call: LastNode 0 -> next node index 1 (n2) gets the low score.
	scores := r.Score(&task.Task{}, nodes)

	if scores["n2"] != 0.1 {
		t.Errorf("expected n2 to score 0.1, got %v", scores["n2"])
	}
	if scores["n1"] != 1.0 || scores["n3"] != 1.0 {
		t.Errorf("expected n1/n3 to score 1.0, got n1=%v n3=%v", scores["n1"], scores["n3"])
	}
	if r.LastNode != 1 {
		t.Errorf("expected LastNode 1, got %d", r.LastNode)
	}
}

func TestScoreWrapsAround(t *testing.T) {
	r := &RoundRobin{Name: "rr"}
	nodes := testNodes("n1", "n2")

	// LastNode starts 0 -> picks index 1, then LastNode 1 is last -> wraps to 0.
	r.Score(&task.Task{}, nodes) // LastNode -> 1
	scores := r.Score(&task.Task{}, nodes)

	if scores["n1"] != 0.1 {
		t.Errorf("expected wrap to favor n1 (0.1), got %v", scores["n1"])
	}
	if r.LastNode != 0 {
		t.Errorf("expected LastNode reset to 0, got %d", r.LastNode)
	}
}

func TestPickLowestScore(t *testing.T) {
	r := &RoundRobin{Name: "rr"}
	nodes := testNodes("n1", "n2", "n3")
	scores := map[string]float64{"n1": 1.0, "n2": 0.1, "n3": 1.0}

	got := r.Pick(scores, nodes)

	if got == nil || got.Name != "n2" {
		t.Fatalf("expected n2 picked, got %v", got)
	}
}

func TestScoreThenPickSelectsNextNode(t *testing.T) {
	r := &RoundRobin{Name: "rr"}
	nodes := testNodes("n1", "n2", "n3")

	scores := r.Score(&task.Task{}, nodes)
	got := r.Pick(scores, nodes)

	if got.Name != "n2" {
		t.Errorf("expected n2 selected, got %s", got.Name)
	}
}
