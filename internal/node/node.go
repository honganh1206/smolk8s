package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/honganh1206/smolk8s/internal/metrics"
	"github.com/honganh1206/smolk8s/internal/utils"
)

// Node represents any machine in the cluster
type Node struct {
	Name            string
	API             string
	IP              string
	Cores           int
	Memory          int64
	MemoryAllocated int64
	Disk            int64
	DiskAllocated   int64
	Stats           *metrics.Stats
	Role            string
	TaskCount       int
}

func New(name, api, role string) *Node {
	return &Node{
		Name: name,
		API:  api,
		Role: role,
	}
}

func (n *Node) GetStats() (*metrics.Stats, error) {
	var resp *http.Response
	var err error

	url := fmt.Sprintf("%s/stats", n.API)
	resp, err = utils.RetryHTTP(http.Get, url)
	if err != nil {
		msg := fmt.Sprintf("Unable to connect to %v. Permanent failure.\n", n.API)
		log.Println(msg)
		return nil, errors.New(msg)
	}

	if resp.StatusCode != 200 {
		msg := fmt.Sprintf("Error retrieving stats from %v: %v", n.API, err)
		log.Println(msg)
		return nil, errors.New(msg)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		msg := fmt.Sprintf("Unable to read response body from %v.\n", n.API)
		log.Println(msg)
		return nil, errors.New(msg)

	}

	var stats *metrics.Stats
	err = json.Unmarshal(body, &stats)
	if err != nil {
		msg := fmt.Sprintf("error decoding message while getting stats for node %s", n.Name)
		log.Println(msg)
		return nil, errors.New(msg)
	}

	if stats.MemStats == nil || stats.DiskStats == nil {
		return nil, fmt.Errorf("error getting stats from node %s", n.Name)
	}

	n.Memory = int64(stats.MemTotalKb())
	n.Disk = int64(stats.DiskTotal())
	n.Stats = stats

	return n.Stats, nil
}
