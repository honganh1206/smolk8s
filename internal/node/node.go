package node

// Node represents any machine in the cluster
type Node struct {
	Name            string
	API             string
	IP              string
	Cores           int
	Memory          int
	MemoryAllocated int
	Disk            int
	DiskAllocated   int
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
