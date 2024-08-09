package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

const CmdTimeout = time.Duration(5 * time.Second)

type SensorValue struct {
	Name  string
	Value float64
}

type SensorInfo struct {
	Name   string
	Type   string
	Values []SensorValue
}

func GetSensorInfos() ([]SensorInfo, error) {
	out, err := Exec("sensors", "-A", "-j")
	if err != nil {
		return nil, fmt.Errorf("failed to run command sensors: %w - %s", err, string(out))
	}
	var outJson *map[string](map[string](map[string]float64))
	json.Unmarshal(out, &outJson)
	var sensorInfos []SensorInfo
	for k, v := range *outJson {
		for kk, vv := range v {
			sensorInfo := SensorInfo{}
			sensorInfo.Name = k + kk
			sensorInfo.Values = []SensorValue{}
			for kkk, vvv := range vv {
				splitted := strings.Split(kkk, "_")

				sensorInfo.Type = regexp.MustCompile("[0-9]+").ReplaceAllString(splitted[0], "")
				sensorInfo.Values = append(sensorInfo.Values, SensorValue{strings.Join(splitted[1:], "_"), vvv})
			}
			sensorInfos = append(sensorInfos, sensorInfo)
		}
	}
	sort.Slice(sensorInfos, func(i, j int) bool {
		return sensorInfos[i].Name < sensorInfos[j].Name
	})
	return sensorInfos, nil
}
