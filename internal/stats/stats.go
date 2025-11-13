package stats

import (
	"log"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
)

type Stats struct {
	MemStats  *mem.VirtualMemoryStat
	DiskStats *disk.UsageStat
	CPUStats  []cpu.TimesStat
	LoadStats *load.AvgStat
	TaskCount int
}

func New() *Stats {
	return &Stats{
		MemStats:  getMemoryInfo(),
		DiskStats: getDiskInfo(),
		CPUStats:  getCPUStats(),
		LoadStats: getLoadAvg(),
	}
}

func getMemoryInfo() *mem.VirtualMemoryStat {
	info, err := mem.VirtualMemory()
	if err != nil {
		log.Printf("Error reading memory info: %v", err)
		return &mem.VirtualMemoryStat{}
	}

	return info
}

func getDiskInfo() *disk.UsageStat {
	usage, err := disk.Usage("/")
	if err != nil {
		log.Printf("Error reading disk usage: %v", err)
		return &disk.UsageStat{}
	}

	return usage
}

func getCPUStats() []cpu.TimesStat {
	stats, err := cpu.Times(false)
	if err != nil {
		log.Printf("Error reading CPU stats: %v", err)
		return []cpu.TimesStat{}
	}

	return stats
}

func getLoadAvg() *load.AvgStat {
	avg, err := load.Avg()
	if err != nil {
		log.Printf("Error reading load avg: %v", err)
		return &load.AvgStat{}
	}
	return avg
}
