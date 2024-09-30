package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type ContainerInfo struct {
	Name    string
	ID      string
	State   string
	Net     string
	Ports   []dockertypes.Port
	Volumes []dockertypes.MountPoint
	CpuLoad float64
	MemLoad float64
}

type ControlInput struct {
	ID  string `json:"id"`
	Opt string `json:"opt"`
}

var dockerCtx = context.Background()
var dockerCli, _ = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

func getContainers() ([]ContainerInfo, error) {
	listOptions := containertypes.ListOptions{}
	listOptions.All = true
	containers, err := dockerCli.ContainerList(dockerCtx, listOptions)
	if err != nil {
		return nil, err
	}
	containerInfos := []ContainerInfo{}

	for _, container := range containers {
		containerInfo := ContainerInfo{}
		containerInfo.Ports = container.Ports
		containerInfo.Name = strings.Join(container.Names, ",")
		containerInfo.ID = container.ID
		for k, _ := range container.NetworkSettings.Networks {
			containerInfo.Net += k
		}
		containerInfo.State = container.State
		containerInfo.Volumes = container.Mounts
		if containerInfo.State != "running" {
			containerInfos = append(containerInfos, containerInfo)
			continue
		}
		statRow, err := dockerCli.ContainerStatsOneShot(dockerCtx, container.ID)
		if err != nil {
			containerInfos = append(containerInfos, containerInfo)
			continue
		}
		statInfo, err := io.ReadAll(statRow.Body)
		statRow.Body.Close()
		if err != nil {
			containerInfos = append(containerInfos, containerInfo)
			continue
		}
		statInfoStr := string(statInfo)
		memUsage := gjson.Get(statInfoStr, "memory_stats.usage").Num
		memLimit := gjson.Get(statInfoStr, "memory_stats.limit").Num
		if memLimit <= 0 {
			containerInfo.MemLoad = 0
		} else {
			containerInfo.MemLoad = memUsage / memLimit * 100
		}
		cpuUsage := gjson.Get(statInfoStr, "cpu_stats.cpu_usage.total_usage").Num
		systemCpuUsage := gjson.Get(statInfoStr, "cpu_stats.system_cpu_usage").Num
		preCpuUsage := gjson.Get(statInfoStr, "precpu_stats.cpu_usage.total_usage").Num
		preCystemCpuUsage := gjson.Get(statInfoStr, "precpu_stats.system_cpu_usage").Num
		if systemCpuUsage-preCystemCpuUsage <= 0 {
			containerInfo.CpuLoad = 0
		} else {
			containerInfo.CpuLoad = (cpuUsage - preCpuUsage) / (systemCpuUsage - preCystemCpuUsage) * 100
		}
		containerInfos = append(containerInfos, containerInfo)
	}
	return containerInfos, nil
}

func controlContainer(c *gin.Context) {
	var controlInput ControlInput
	if err := c.ShouldBindJSON(&controlInput); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	containerId := controlInput.ID
	opt := controlInput.Opt
	var err error
	switch opt {
	case "restart":
		err = dockerCli.ContainerRestart(dockerCtx, containerId, containertypes.StopOptions{})
	case "stop":
		err = dockerCli.ContainerStop(dockerCtx, containerId, containertypes.StopOptions{})
	case "start":
		err = dockerCli.ContainerStart(dockerCtx, containerId, containertypes.StartOptions{})
	case "pause":
		err = dockerCli.ContainerPause(dockerCtx, containerId)
	case "unpause":
		err = dockerCli.ContainerUnpause(dockerCtx, containerId)
	}
	if err != nil {
		log.Print(err)
		c.JSON(500, gin.H{"type": "error", "message": err.Error()})
		return
	}
	catch_5s(&catch_5sData)
	// for i, container := range catch_5sData.Containers {
	// 	if container.ID == containerId {

	// 		listOptions := containertypes.ListOptions{}
	// 		listOptions.All = true
	// 		listOptions.Filters = filters.NewArgs()
	// 		listOptions.Filters.Add("id", containerId)
	// 		containers, err := dockerCli.ContainerList(dockerCtx, listOptions)
	// 		if err != nil {
	// 			break
	// 		}
	// 		catch_5sData.Containers[i].State = containers[0].State
	// 		log.Print(catch_5sData)
	// 	}
	// }
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": fmt.Sprintf("%s container %s succeed", opt, containerId)})
}

func addContainer(ctx *gin.Context) {

}

func test() {
	statRow, err := dockerCli.ContainerStats(dockerCtx, "1c329d0609f7632a0f67b6a4eb8160db1760be4e3eac254185da74e1423d6898", false)
	if err != nil {
		return
	}
	statInfo, err := io.ReadAll(statRow.Body)
	if err != nil {
		return
	}
	log.Print(string(statInfo))
	time.Sleep(time.Second)
	statInfo, err = io.ReadAll(statRow.Body)
	if err != nil {
		return
	}
	log.Print(string(statInfo))
}
