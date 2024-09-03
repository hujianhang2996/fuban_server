package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	dockertypes "github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
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

var dockerCtx = context.Background()
var dockerCli, _ = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

func getContainers() ([]ContainerInfo, error) {

	defer dockerCli.Close()
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

		statRow, err := dockerCli.ContainerStats(dockerCtx, container.ID, false)
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
		var v map[string]interface{}
		json.Unmarshal(statInfo, &v)
		memStat := v["memory_stats"].(map[string]interface{})
		if len(memStat) > 0 {
			containerInfo.MemLoad = memStat["usage"].(float64) / memStat["limit"].(float64) * 100
		}
		containerInfos = append(containerInfos, containerInfo)
	}

	return containerInfos, nil

}

func controlContainer(ctx *gin.Context) {
	containerId := ctx.PostForm("id")
	opt := ctx.PostForm("opt")
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
		ctx.JSON(500, gin.H{"type": "error", "message": err.Error()})
		return
	}
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
	ctx.JSON(http.StatusOK, gin.H{"type": "success", "message": fmt.Sprintf("%s container %s succeed", opt, containerId)})
}

func test() {
	statRow, err := dockerCli.ContainerStats(dockerCtx, "2c28528287d20ae7f5589ff15dd1b37ba0842f80ac684b21800b4a0f7a2e4c88", false)
	if err != nil {
		return
	}
	statInfo, err := io.ReadAll(statRow.Body)
	if err != nil {
		return
	}
	defer statRow.Body.Close()
	var v map[string]interface{}
	json.Unmarshal(statInfo, &v)
	MemLoad := v["memory_stats"].(map[string]interface{})["usage"].(float64) / v["memory_stats"].(map[string]interface{})["limit"].(float64) * 100
	log.Print(MemLoad)
}
