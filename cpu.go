package main

import (
	"fmt"
	"math"

	"github.com/shirou/gopsutil/v4/cpu"
)

type CpuTime struct {
	Total  float64
	Active float64
}

func getCpuLoads(oldCpuTime *map[string]CpuTime) (map[string]int64, error) {
	cpuUsages := make(map[string]int64)
	timeStats, err := CPUTimes()
	if err != nil {
		return cpuUsages, fmt.Errorf("error getting CPU info: %w", err)
	}
	for _, timeStat := range timeStats {
		v, ok := (*oldCpuTime)[timeStat.CPU]
		newTotal := totalCPUTime(timeStat)
		newActive := activeCPUTime(timeStat)
		if ok {
			cpuUsages[timeStat.CPU] = int64(math.Round((newActive - v.Active) / (newTotal - v.Total) * 100))
		} else {
			cpuUsages[timeStat.CPU] = 0
		}
		(*oldCpuTime)[timeStat.CPU] = CpuTime{newTotal, newActive}
	}
	return cpuUsages, nil
}

func CPUTimes() ([]cpu.TimesStat, error) {
	var cpuTimes []cpu.TimesStat
	perCPUTimes, err := cpu.Times(true)
	if err != nil {
		return nil, err
	}
	cpuTimes = append(cpuTimes, perCPUTimes...)
	totalCPUTimes, err := cpu.Times(false)
	if err != nil {
		return nil, err
	}
	cpuTimes = append(cpuTimes, totalCPUTimes...)
	return cpuTimes, nil
}

func totalCPUTime(t cpu.TimesStat) float64 {
	total := t.User + t.System + t.Nice + t.Iowait + t.Irq + t.Softirq + t.Steal + t.Idle
	return total
}

func activeCPUTime(t cpu.TimesStat) float64 {
	active := totalCPUTime(t) - t.Idle
	return active
}

// func main() {
// 	oldCpuTimeStat := make(map[string]CpuTime)

// 	ticker := time.NewTicker(time.Millisecond * 100)
// 	for t := range ticker.C {
// 		cpuLoads, err := getCpuLoads(&oldCpuTimeStat)
// 		if err != nil {
// 			continue
// 		}
// 		log.Println(t, cpuLoads["cpu-total"])
// 	}
// }
