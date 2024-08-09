package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const scalingFactor = float64(1000.0)

type TemperatureStat struct {
	Name        string
	Label       string
	Device      string
	Temperature float64
	Additional  map[string]interface{}
}

func GetTemps() (map[string]TemperatureStat, error) {
	path := os.Getenv("HOST_SYS")
	if path == "" {
		path = "/sys"
	}
	temperatures, err := gatherHwmon(path)
	if err != nil {
		return temperatures, fmt.Errorf("getting temperatures failed: %w", err)
	}

	if len(temperatures) == 0 {
		temperatures, err = gatherThermalZone(path)
		if err != nil {
			return temperatures, fmt.Errorf("getting temperatures (via fallback) failed: %w", err)
		}
	}
	return temperatures, nil
}

func gatherHwmon(syspath string) (map[string]TemperatureStat, error) {
	sensors, err := filepath.Glob(filepath.Join(syspath, "class", "hwmon", "hwmon*", "temp*_input"))
	if err != nil {
		return nil, fmt.Errorf("getting sensors failed: %w", err)
	}
	if len(sensors) == 0 {
		sensors, err = filepath.Glob(filepath.Join(syspath, "class", "hwmon", "hwmon*", "device", "temp*_input"))
		if err != nil {
			return nil, fmt.Errorf("getting sensors on CentOS failed: %w", err)
		}
	}
	if len(sensors) == 0 {
		return nil, nil
	}
	stats := make(map[string]TemperatureStat)
	for _, s := range sensors {
		path := filepath.Dir(s)
		prefix := strings.SplitN(filepath.Base(s), "_", 2)[0]

		deviceName, err := os.Readlink(filepath.Join(path, "device"))
		if err == nil {
			deviceName = filepath.Base(deviceName)
		}

		name := deviceName
		n, err := os.ReadFile(filepath.Join(path, "name"))
		if err == nil {
			name = strings.TrimSpace(string(n))
		}

		var label string
		if buf, err := os.ReadFile(filepath.Join(path, prefix+"_label")); err == nil {
			label = strings.TrimSpace(string(buf))
		}

		temp := TemperatureStat{
			Name:       name,
			Label:      strings.ToLower(label),
			Device:     deviceName,
			Additional: make(map[string]interface{}),
		}

		// Temperature (mandatory)
		fn := filepath.Join(path, prefix+"_input")
		buf, err := os.ReadFile(fn)
		if err != nil {
			continue
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(string(buf)), 64); err == nil {
			temp.Temperature = v / scalingFactor
		}

		// Read all possible values of the sensor
		matches, err := filepath.Glob(filepath.Join(path, prefix+"_*"))
		if err != nil {
			continue
		}
		for _, fn := range matches {
			buf, err = os.ReadFile(fn)
			if err != nil {
				continue
			}
			parts := strings.SplitN(filepath.Base(fn), "_", 2)
			if len(parts) != 2 {
				continue
			}
			measurement := parts[1]

			switch measurement {
			case "label", "input":
				continue
			}

			v, err := strconv.ParseFloat(strings.TrimSpace(string(buf)), 64)
			if err != nil {
				continue
			}
			temp.Additional[measurement] = v / scalingFactor
		}
		stats[temp.Name+"-"+temp.Label+"-"+temp.Device] = temp
	}

	return stats, nil
}

func gatherThermalZone(syspath string) (map[string]TemperatureStat, error) {
	zones, err := filepath.Glob(filepath.Join(syspath, "class", "thermal", "thermal_zone*"))
	if err != nil {
		return nil, fmt.Errorf("getting thermal zones failed: %w", err)
	}

	if len(zones) == 0 {
		return nil, nil
	}

	stats := make(map[string]TemperatureStat)
	for _, path := range zones {
		buf, err := os.ReadFile(filepath.Join(path, "type"))
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(buf))

		buf, err = os.ReadFile(filepath.Join(path, "temp"))
		if err != nil {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(string(buf)), 64)
		if err != nil {
			continue
		}

		temp := TemperatureStat{Name: name, Temperature: v / scalingFactor}
		stats[temp.Name+"-"+temp.Label+"-"+temp.Device] = temp
	}

	return stats, nil
}
