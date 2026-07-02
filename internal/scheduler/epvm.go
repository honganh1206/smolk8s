package scheduler

import (
	"log"
	"math"
	"time"

	"github.com/honganh1206/smolk8s/internal/node"
	"github.com/honganh1206/smolk8s/internal/task"
)

const (
	// LIEB square ice constant (it's just a nice-enough base)
	// https://en.wikipedia.org/wiki/Lieb%27s_square_ice_constant
	// Analogy: A huge checkerboard, water molecules at every intersection.
	// Each molecule must satisfy the ice rule: 2 bonds in, 2 bonds out
	// Imagine we have N molecules, that means there will be 1.5396^N arragements
	LIEB = 1.53960071783900203869
)

type EPVM struct {
	Name string
}

func (e *EPVM) SelectCandidateNodes(t *task.Task, nodes []*node.Node) []*node.Node {
	var candidates []*node.Node
	for node := range nodes {
		if checkDisk(t, nodes[node].Disk-nodes[node].DiskAllocated) {
			candidates = append(candidates, nodes[node])
		}
	}

	return candidates
}

func (e *EPVM) Score(t *task.Task, nodes []*node.Node) map[string]float64 {
	nodeScores := make(map[string]float64)
	maxJobs := 4.0
	// Possible highest load seen on any of the nodes;
	// we can use any base, not just 2.
	cpuBase := math.Pow(2, 0.8)

	for _, n := range nodes {
		cpuUsage, err := calculateCPUUsage(n)
		if err != nil {
			log.Printf("error calculating CPU usage for node %s, skipping: %v", n.Name, err)
			continue
		}

		cpuLoad := calculateLoad(*cpuUsage, cpuBase)
		newCpuLoad := calculateLoad(*cpuUsage+t.CPU, cpuBase)

		memoryAllocated := float64(n.Stats.MemUsedKb()) + float64(n.MemoryAllocated)
		memoryPercentAllocated := memoryAllocated / float64(n.Memory)
		newMemPercent := calculateLoad(memoryAllocated+float64(t.Memory/1000), float64(n.Memory))

		newTaskCount := taskCountCost(n.TaskCount+1, maxJobs)
		oldTaskCount := taskCountCost(n.TaskCount, maxJobs)

		memCost := math.Pow(LIEB, newMemPercent) + newTaskCount -
			math.Pow(LIEB, memoryPercentAllocated) - oldTaskCount

		cpuCost := math.Pow(LIEB, newCpuLoad) + newTaskCount -
			math.Pow(LIEB, cpuLoad) - oldTaskCount

		nodeScores[n.Name] = memCost + cpuCost
	}

	return nodeScores
}

func (e *EPVM) Pick(scores map[string]float64, candidates []*node.Node) *node.Node {
	minCost := 0.00
	var bestNode *node.Node
	for idx, node := range candidates {
		if idx == 0 {
			minCost = scores[node.Name]
			bestNode = node
			continue
		}

		if scores[node.Name] < minCost {
			minCost = scores[node.Name]
			bestNode = node
		}
	}
	return bestNode
}

// taskCountCost returns the LIEB-based cost of a node holding count tasks,
// normalized by maxJobs. Shared by memCost and cpuCost in EPVM.Score.
func taskCountCost(count int, maxJobs float64) float64 {
	return math.Pow(LIEB, float64(count)/maxJobs)
}

func checkDisk(t *task.Task, diskAvailable int64) bool {
	return t.Disk < diskAvailable
}

// Refer to: https://stackoverflow.com/questions/23367857/accurate-calculation-of-cpu-usage-given-in-percentage-in-linux
func calculateCPUUsage(node *node.Node) (*float64, error) {
	stat1, err := node.GetStats()
	if err != nil {
		return nil, err
	}

	// Record delta between two snapshots
	time.Sleep(3 * time.Second)

	stat2, err := node.GetStats()
	if err != nil {
		return nil, err
	}

	stat2Idle := stat2.CPUStats.Idle + stat2.CPUStats.IOWait
	stat1Idle := stat1.CPUStats.Idle + stat1.CPUStats.IOWait

	stat1NonIdle := stat1.CPUStats.User +
		// Time if processes with "nice" priority
		stat1.CPUStats.Nice +
		stat1.CPUStats.System +
		// Interruption request
		stat1.CPUStats.IRQ +
		stat1.CPUStats.SoftIRQ +
		// Time stolen by the hypervisor
		stat1.CPUStats.Steal

	stat2NonIdle := stat2.CPUStats.User +
		stat2.CPUStats.Nice +
		stat2.CPUStats.System +
		stat2.CPUStats.IRQ +
		stat2.CPUStats.SoftIRQ +
		stat2.CPUStats.Steal

	stat1Total := stat1Idle + stat1NonIdle
	stat2Total := stat2Idle + stat2NonIdle

	total := stat2Total - stat1Total
	idle := stat2Idle - stat1Idle

	var cpuPercentUsage float64
	if total == 0 && idle == 0 {
		cpuPercentUsage = 0.00
	} else {
		cpuPercentUsage = (float64(total) - float64(idle)) / float64(total)
	}
	return &cpuPercentUsage, nil
}

func calculateLoad(usage, capacity float64) float64 {
	return usage / capacity
}
