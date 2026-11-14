// Package hoststats reads host CPU and memory usage from /proc. On a system
// without /proc (development on Windows or macOS) it reports zeros.
package hoststats

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Snapshot is the node's current load.
type Snapshot struct {
	CPUPercent float64 // 0-100 across all cores
	UsedRAMMB  float64
	TotalRAMMB float64
}

type cpuTimes struct {
	idle  uint64
	total uint64
}

// Sampler measures CPU usage between successive calls.
type Sampler struct {
	mu       sync.Mutex
	previous *cpuTimes

	statPath    string
	meminfoPath string
}

func NewSampler() *Sampler {
	return &Sampler{statPath: "/proc/stat", meminfoPath: "/proc/meminfo"}
}

// Sample returns usage since the previous call. The first call reports no
// CPU usage because there is nothing to compare against yet.
func (s *Sampler) Sample() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	var snapshot Snapshot
	if current, ok := readCPUTimes(s.statPath); ok {
		if s.previous != nil && current.total > s.previous.total {
			total := float64(current.total - s.previous.total)
			idle := float64(current.idle - s.previous.idle)
			if busy := (total - idle) / total * 100; busy > 0 {
				snapshot.CPUPercent = min(busy, 100)
			}
		}
		s.previous = &current
	}

	snapshot.UsedRAMMB, snapshot.TotalRAMMB = readMemory(s.meminfoPath)
	return snapshot
}

func readCPUTimes(path string) (cpuTimes, bool) {
	file, err := os.Open(path)
	if err != nil {
		return cpuTimes{}, false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 || fields[0] != "cpu" {
			continue
		}

		var times cpuTimes
		for index, field := range fields[1:] {
			value, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				return cpuTimes{}, false
			}
			// Guest time is already included in user time.
			if index >= 8 {
				break
			}
			times.total += value
			// Fields 3 and 4 are idle and iowait.
			if index == 3 || index == 4 {
				times.idle += value
			}
		}
		return times, true
	}
	return cpuTimes{}, false
}

func readMemory(path string) (usedMB float64, totalMB float64) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	values := map[string]float64{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, rest, found := strings.Cut(scanner.Text(), ":")
		if !found {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		if value, err := strconv.ParseFloat(fields[0], 64); err == nil {
			values[key] = value // kB
		}
	}

	total, hasTotal := values["MemTotal"]
	available, hasAvailable := values["MemAvailable"]
	if !hasTotal || !hasAvailable || total <= 0 {
		return 0, 0
	}
	return (total - available) / 1024, total / 1024
}
