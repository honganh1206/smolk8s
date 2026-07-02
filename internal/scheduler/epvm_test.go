package scheduler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/honganh1206/smolk8s/internal/node"
	"github.com/honganh1206/smolk8s/internal/task"

	"github.com/c9s/goprocinfo/linux"
)

// makeStats builds a *metrics.Stats-equivalent JSON payload with the given CPU
// fields and a fixed memory/disk footprint so that node.GetStats succeeds.
func makeStatsJSON(cpu linux.CPUStat) string {
	type statsPayload struct {
		MemStats  *linux.MemInfo `json:"MemStats"`
		DiskStats *linux.Disk    `json:"DiskStats"`
		CPUStats  *linux.CPUStat `json:"CPUStats"`
		LoadStats *linux.LoadAvg `json:"LoadStats"`
		TaskCount int            `json:"TaskCount"`
	}
	s := statsPayload{
		// MemTotal=1000000, MemAvailable=500000 -> MemUsedKb()=500000
		MemStats: &linux.MemInfo{MemTotal: 1000000, MemAvailable: 500000},
		DiskStats: &linux.Disk{All: 1000000, Used: 500000, Free: 500000, FreeInodes: 1000},
		CPUStats:  &cpu,
	}
	b, _ := json.Marshal(s)
	return string(b)
}

// newStatsServer returns an httptest server that responds to GET /stats with
// two distinct CPU snapshots in sequence (then repeats the second one) so that
// calculateCPUUsage can observe a non-zero delta.
func newStatsServer(stat1, stat2 linux.CPUStat) *httptest.Server {
	calls := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_, _ = w.Write([]byte(makeStatsJSON(stat1)))
			return
		}
		_, _ = w.Write([]byte(makeStatsJSON(stat2)))
	}))
}

// epvmTestNodes builds nodes with the given disk/memory configuration pointing
// at the provided stats server.
func epvmTestNodes(api string, names ...string) []*node.Node {
	nodes := make([]*node.Node, len(names))
	for i, n := range names {
		nodes[i] = &node.Node{Name: n, API: api}
	}
	return nodes
}

func TestEPVMSelectCandidateNodesFiltersByDisk(t *testing.T) {
	e := &EPVM{Name: "epvm"}
	tk := &task.Task{Disk: 50}

	nodes := []*node.Node{
		{Name: "fits", Disk: 100, DiskAllocated: 0}, // available 100 > 50 -> candidate
		{Name: "tight", Disk: 60, DiskAllocated: 20}, // available 40 < 50 -> filtered
		{Name: "full", Disk: 100, DiskAllocated: 100}, // available 0 < 50 -> filtered
	}

	got := e.SelectCandidateNodes(tk, nodes)

	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d (%+v)", len(got), got)
	}
	if got[0].Name != "fits" {
		t.Fatalf("expected 'fits' to be the only candidate, got %s", got[0].Name)
	}
}

func TestEPVMSelectCandidateNodesEmptyInput(t *testing.T) {
	e := &EPVM{Name: "epvm"}
	got := e.SelectCandidateNodes(&task.Task{Disk: 10}, nil)
	if len(got) != 0 {
		t.Fatalf("expected no candidates for nil input, got %d", len(got))
	}
}

func TestEPVMScoreReturnsScoreForEveryNode(t *testing.T) {
	// CPU snapshot 1: idle=100, non-idle=0 -> total=100
	// CPU snapshot 2: idle=150, non-idle=50 -> total=200
	// total delta=100, idle delta=50 -> usage=(100-50)/100=0.5
	stat1 := linux.CPUStat{Idle: 100}
	stat2 := linux.CPUStat{Idle: 150, User: 50}

	srv := newStatsServer(stat1, stat2)
	defer srv.Close()

	e := &EPVM{Name: "epvm"}
	nodes := epvmTestNodes(srv.URL, "n1", "n2", "n3")
	tk := &task.Task{CPU: 0.1, Memory: 1000, Disk: 10}

	scores := e.Score(tk, nodes)

	if len(scores) != len(nodes) {
		t.Fatalf("expected %d scores, got %d (%+v)", len(nodes), len(scores), scores)
	}
	for _, n := range nodes {
		if _, ok := scores[n.Name]; !ok {
			t.Errorf("missing score for node %s", n.Name)
		}
	}
}

func TestEPVMScoreIdenticalNodesGetIdenticalScores(t *testing.T) {
	// Both nodes hit the same server and get the same CPU deltas, so their
	// scores must be equal.
	stat1 := linux.CPUStat{Idle: 100}
	stat2 := linux.CPUStat{Idle: 150, User: 50}

	srv := newStatsServer(stat1, stat2)
	defer srv.Close()

	e := &EPVM{Name: "epvm"}
	// Two separate servers so each node gets its own stat1/stat2 pair.
	srv2 := newStatsServer(stat1, stat2)
	defer srv2.Close()

	nodes := []*node.Node{
		{Name: "n1", API: srv.URL},
		{Name: "n2", API: srv2.URL},
	}
	tk := &task.Task{CPU: 0.1, Memory: 1000, Disk: 10}

	scores := e.Score(tk, nodes)

	if scores["n1"] != scores["n2"] {
		t.Errorf("expected identical scores for identical nodes, got n1=%v n2=%v", scores["n1"], scores["n2"])
	}
}

func TestEPVMScoreSkipsNodeOnStatsError(t *testing.T) {
	// Server returns 500 -> GetStats fails -> node is skipped (no score entry).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	e := &EPVM{Name: "epvm"}
	nodes := epvmTestNodes(srv.URL, "bad")
	tk := &task.Task{CPU: 0.1, Memory: 1000, Disk: 10}

	scores := e.Score(tk, nodes)

	if len(scores) != 0 {
		t.Fatalf("expected no scores when stats fail, got %d (%+v)", len(scores), scores)
	}
}

func TestCalculateCPUUsageFiftyPercent(t *testing.T) {
	stat1 := linux.CPUStat{Idle: 100}
	stat2 := linux.CPUStat{Idle: 150, User: 50}

	srv := newStatsServer(stat1, stat2)
	defer srv.Close()

	n := &node.Node{Name: "n1", API: srv.URL}
	usage, err := calculateCPUUsage(n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage == nil {
		t.Fatal("expected non-nil usage")
	}
	if *usage != 0.5 {
		t.Errorf("expected 0.5 CPU usage, got %v", *usage)
	}
}

func TestCalculateCPUUsageZeroWhenNoDelta(t *testing.T) {
	// Identical snapshots -> total=0, idle=0 -> branch returns 0.
	stat := linux.CPUStat{Idle: 100, User: 50}

	srv := newStatsServer(stat, stat)
	defer srv.Close()

	n := &node.Node{Name: "n1", API: srv.URL}
	usage, err := calculateCPUUsage(n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *usage != 0.0 {
		t.Errorf("expected 0.0 CPU usage for identical snapshots, got %v", *usage)
	}
}

func TestCalculateCPUUsageErrorOnFailedStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := &node.Node{Name: "n1", API: srv.URL}
	usage, err := calculateCPUUsage(n)
	if err == nil {
		t.Fatal("expected error when GetStats fails, got nil")
	}
	if usage != nil {
		t.Errorf("expected nil usage on error, got %v", *usage)
	}
}

func TestCalculateLoad(t *testing.T) {
	cases := []struct {
		usage, capacity, want float64
	}{
		{0, 1, 0},
		{1, 2, 0.5},
		{3, 4, 0.75},
		{2, 2, 1},
	}
	for _, c := range cases {
		got := calculateLoad(c.usage, c.capacity)
		if got != c.want {
			t.Errorf("calculateLoad(%v, %v) = %v, want %v", c.usage, c.capacity, got, c.want)
		}
	}
}

func TestTaskCountCost(t *testing.T) {
	// count=0 -> LIEB^0 = 1; count=maxJobs -> LIEB^1 = LIEB.
	if got := taskCountCost(0, 4.0); got != 1.0 {
		t.Errorf("taskCountCost(0,4) = %v, want 1.0", got)
	}
	if got := taskCountCost(4, 4.0); got != LIEB {
		t.Errorf("taskCountCost(4,4) = %v, want %v", got, LIEB)
	}
	// More tasks -> higher cost (LIEB > 1, exponent increasing).
	if taskCountCost(2, 4.0) >= taskCountCost(3, 4.0) {
		t.Errorf("expected cost to increase with task count")
	}
}

func TestCheckDisk(t *testing.T) {
	tk := &task.Task{Disk: 50}
	if !checkDisk(tk, 100) {
		t.Errorf("checkDisk(Disk=50, available=100) = false, want true")
	}
	if checkDisk(tk, 50) {
		t.Errorf("checkDisk(Disk=50, available=50) = true, want false (task.Disk must be < available)")
	}
	if checkDisk(tk, 10) {
		t.Errorf("checkDisk(Disk=50, available=10) = true, want false")
	}
}

func TestEPVMPickLowestScore(t *testing.T) {
	e := &EPVM{Name: "epvm"}
	nodes := testNodes("n1", "n2", "n3")
	scores := map[string]float64{"n1": 1.5, "n2": 0.2, "n3": 0.9}

	got := e.Pick(scores, nodes)

	if got == nil {
		t.Fatal("expected a node, got nil")
	}
	if got.Name != "n2" {
		t.Errorf("expected n2 (lowest score), got %s", got.Name)
	}
}

func TestEPVMPickFirstCandidateLowest(t *testing.T) {
	// Lowest score is on the very first candidate.
	e := &EPVM{Name: "epvm"}
	nodes := testNodes("n1", "n2", "n3")
	scores := map[string]float64{"n1": 0.1, "n2": 0.5, "n3": 0.9}

	got := e.Pick(scores, nodes)

	if got == nil || got.Name != "n1" {
		t.Fatalf("expected n1, got %v", got)
	}
}

func TestEPVMPickResolvesTieToFirst(t *testing.T) {
	// Two candidates share the lowest score; the loop uses strict `<`, so the
	// first one encountered must win.
	e := &EPVM{Name: "epvm"}
	nodes := testNodes("n1", "n2", "n3")
	scores := map[string]float64{"n1": 0.5, "n2": 0.5, "n3": 1.0}

	got := e.Pick(scores, nodes)

	if got == nil || got.Name != "n1" {
		t.Fatalf("expected n1 on tie, got %v", got)
	}
}

func TestEPVMPickSingleCandidate(t *testing.T) {
	e := &EPVM{Name: "epvm"}
	nodes := testNodes("solo")
	scores := map[string]float64{"solo": 0.42}

	got := e.Pick(scores, nodes)

	if got == nil || got.Name != "solo" {
		t.Fatalf("expected solo, got %v", got)
	}
}

func TestEPVMPickEmptyReturnsNil(t *testing.T) {
	e := &EPVM{Name: "epvm"}
	got := e.Pick(map[string]float64{}, nil)
	if got != nil {
		t.Errorf("expected nil for empty candidates, got %v", got)
	}
}
