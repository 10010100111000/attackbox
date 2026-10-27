package node

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func hasKVMSupport() bool {
	if _, err := os.Stat("/dev/kvm"); err == nil {
		return true
	}
	return false
}

// readSystemMemoryMB reads real Total and Available physical memory from /proc/meminfo.
func readSystemMemoryMB() (totalMB int64, availableMB int64) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				totalMB = kb / 1024
			}
		} else if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				availableMB = kb / 1024
			}
		}
		if totalMB > 0 && availableMB > 0 {
			break
		}
	}
	return totalMB, availableMB
}

// readSystemLoadAvg reads real 1-minute system load average from /proc/loadavg.
func readSystemLoadAvg() float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0.0
	}
	fields := strings.Fields(string(data))
	if len(fields) > 0 {
		val, _ := strconv.ParseFloat(fields[0], 64)
		return val
	}
	return 0.0
}
