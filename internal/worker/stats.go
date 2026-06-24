package worker

import (
	"log"

	"github.com/c9s/goprocinfo/linux"
)

type Stats struct {
	MemStats  *linux.MemInfo
	DiskStats *linux.Disk
	// Information about processes running on the system
	CPUStats *linux.CPUStat
	// System's load average
	LoadStats *linux.LoadAvg
	TaskCount int
}

func (s *Stats) MemTotalKb() uint64 {
	return s.MemStats.MemTotal
}

func (s *Stats) MemAvailableKb() uint64 {
	return s.MemStats.MemAvailable
}

func (s *Stats) MemUsedKb() uint64 {
	return s.MemStats.MemTotal - s.MemStats.MemAvailable
}

func (s *Stats) MemUsedPercent() uint64 {
	return s.MemStats.MemAvailable / s.MemStats.MemTotal
}

func (s *Stats) DiskTotal() uint64 {
	return s.DiskStats.All
}

func (s *Stats) DiskFree() uint64 {
	return s.DiskStats.Free
}

func (s *Stats) DiskUsed() uint64 {
	return s.DiskStats.Used
}

// CPUUsage to get CPU usage as a percentage
func (s *Stats) CPUUsage() float64 {
	idle := s.CPUStats.Idle + s.CPUStats.IOWait
	nonIdle := s.CPUStats.User +
		// Running user-space processes with modified "nice" priority
		// i.e. a process is nicer to other processes by lowering its scheduling priority
		s.CPUStats.Nice +
		// Running kernel code
		s.CPUStats.System +
		// Time servicing hardware interruptions
		s.CPUStats.IRQ +
		// Software interrupts
		s.CPUStats.SoftIRQ +
		// Time "stolen" by the hypervisor when running a VM,
		// as the VM might want CPU time but another VM got it
		s.CPUStats.Steal

	total := idle + nonIdle

	if total == 0 {
		return 0.00
	}

	return (float64(total) - float64(idle)) / float64(total)
}

func GetStats() *Stats {
	return &Stats{
		MemStats:  GetMemoryInfo(),
		DiskStats: GetDiskInfo(),
		CPUStats:  GetCPUStats(),
		LoadStats: GetLoadAvg(),
	}
}

func GetMemoryInfo() *linux.MemInfo {
	memstats, err := linux.ReadMemInfo("/proc/meminfo")
	if err != nil {
		log.Printf("Error reading from /proc/meminfo")
		return &linux.MemInfo{}
	}

	return memstats
}

// GetDiskInfo See https://godoc.org/github.com/c9s/goprocinfo/linux#Disk
func GetDiskInfo() *linux.Disk {
	diskstats, err := linux.ReadDisk("/")
	if err != nil {
		log.Printf("Error reading from /")
		return &linux.Disk{}
	}

	return diskstats
}

// GetCPUStats See https://godoc.org/github.com/c9s/goprocinfo/linux#CPUStat
func GetCPUStats() *linux.CPUStat {
	stats, err := linux.ReadStat("/proc/stat")
	if err != nil {
		log.Printf("Error reading from /proc/stat")
		return &linux.CPUStat{}
	}

	return &stats.CPUStatAll
}

// GetLoadAvg See https://godoc.org/github.com/c9s/goprocinfo/linux#LoadAvg
func GetLoadAvg() *linux.LoadAvg {
	loadavg, err := linux.ReadLoadAvg("/proc/loadavg")
	if err != nil {
		log.Printf("Error reading from /proc/loadavg")
		return &linux.LoadAvg{}
	}
	return loadavg
}
