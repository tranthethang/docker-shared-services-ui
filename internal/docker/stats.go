package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// ContainerStat holds CPU and memory telemetry for a single running container.
type ContainerStat struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	CPUPct        float64 `json:"cpuPct"`
	MemUsedBytes  uint64  `json:"memUsedBytes"`
	MemLimitBytes uint64  `json:"memLimitBytes"`
	MemPct        float64 `json:"memPct"`
}

// Capacity describes the Docker daemon / VM CPU and memory capacity.
type Capacity struct {
	NCPU          int    `json:"ncpu"`
	MemTotalBytes uint64 `json:"memTotalBytes"`
}

// DiskUsage summarizes Docker disk consumption and reclaimable space in bytes.
type DiskUsage struct {
	ImagesBytes      uint64 `json:"imagesBytes"`
	ContainersBytes  uint64 `json:"containersBytes"`
	VolumesBytes     uint64 `json:"volumesBytes"`
	BuildCacheBytes  uint64 `json:"buildCacheBytes"`
	TotalBytes       uint64 `json:"totalBytes"`
	ReclaimableBytes uint64 `json:"reclaimableBytes"`
}

// Telemetry combines Docker VM capacity, disk usage, and live container stats.
type Telemetry struct {
	Capacity      Capacity                 `json:"capacity"`
	DiskUsage     DiskUsage                `json:"diskUsage"`
	Containers    map[string]ContainerStat `json:"containers"`
	TotalCPUPct   float64                  `json:"totalCpuPct"`
	TotalMemBytes uint64                   `json:"totalMemBytes"`
}

// CalculateCPUPercent computes container CPU usage percentage safely against zero deltas.
func CalculateCPUPercent(stats container.StatsResponse) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(stats.CPUStats.SystemUsage) - float64(stats.PreCPUStats.SystemUsage)
	onlineCPUs := float64(stats.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if cpuDelta <= 0 || sysDelta <= 0 || onlineCPUs <= 0 {
		return 0
	}
	pct := (cpuDelta / sysDelta) * onlineCPUs * 100.0
	if math.IsNaN(pct) || math.IsInf(pct, 0) {
		return 0
	}
	return pct
}

// CalculateMemUsage computes working-set memory bytes, limit, and percentage safely.
func CalculateMemUsage(mem container.MemoryStats) (used, limit uint64, pct float64) {
	used = mem.Usage
	if cache, ok := mem.Stats["inactive_file"]; ok && cache <= used {
		used -= cache
	} else if cache, ok := mem.Stats["cache"]; ok && cache <= used {
		used -= cache
	}
	limit = mem.Limit
	if limit > 0 {
		pct = (float64(used) / float64(limit)) * 100.0
		if math.IsNaN(pct) || math.IsInf(pct, 0) {
			pct = 0
		}
	}
	return used, limit, pct
}

// CollectContainerStats fetches one-shot CPU and memory stats for running containers.
func CollectContainerStats(ctx context.Context, cli StatsCollector) (map[string]ContainerStat, error) {
	running, err := cli.ContainerList(ctx, container.ListOptions{All: false})
	if err != nil {
		return nil, fmt.Errorf("list running containers: %w", err)
	}

	out := make(map[string]ContainerStat, len(running))
	for _, c := range running {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		stat, ok, err := readOneShotStat(ctx, cli, c)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		if ok {
			out[stat.Name] = stat
		}
	}
	return out, nil
}

func readOneShotStat(ctx context.Context, cli StatsCollector, c container.Summary) (ContainerStat, bool, error) {
	resp, err := cli.ContainerStatsOneShot(ctx, c.ID)
	if err != nil {
		return ContainerStat{}, false, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var decoded container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil && err != io.EOF {
		return ContainerStat{}, false, err
	}

	name := strings.TrimPrefix(decoded.Name, "/")
	if name == "" && len(c.Names) > 0 {
		name = strings.TrimPrefix(c.Names[0], "/")
	}
	if name == "" {
		name = c.ID
	}

	used, limit, memPct := CalculateMemUsage(decoded.MemoryStats)
	return ContainerStat{
		ID:            c.ID,
		Name:          name,
		CPUPct:        CalculateCPUPercent(decoded),
		MemUsedBytes:  used,
		MemLimitBytes: limit,
		MemPct:        memPct,
	}, true, nil
}

// CollectCapacity reads Docker daemon CPU count and total memory.
func CollectCapacity(ctx context.Context, cli StatsCollector) (Capacity, error) {
	info, err := cli.Info(ctx)
	if err != nil {
		return Capacity{}, fmt.Errorf("docker info: %w", err)
	}
	var mem uint64
	if info.MemTotal > 0 {
		mem = uint64(info.MemTotal)
	}
	ncpu := info.NCPU
	if ncpu < 0 {
		ncpu = 0
	}
	return Capacity{
		NCPU:          ncpu,
		MemTotalBytes: mem,
	}, nil
}

// CollectDiskUsage reads Docker disk usage across images, containers, volumes, and build cache.
func CollectDiskUsage(ctx context.Context, cli StatsCollector) (DiskUsage, error) {
	du, err := cli.DiskUsage(ctx, types.DiskUsageOptions{})
	if err != nil {
		return DiskUsage{}, fmt.Errorf("docker disk usage: %w", err)
	}

	var out DiskUsage
	for _, img := range du.Images {
		if img == nil || img.Size <= 0 {
			continue
		}
		size := uint64(img.Size)
		out.ImagesBytes += size
		if img.Containers == 0 {
			out.ReclaimableBytes += size
		}
	}
	for _, ctr := range du.Containers {
		if ctr == nil || ctr.SizeRw <= 0 {
			continue
		}
		size := uint64(ctr.SizeRw)
		out.ContainersBytes += size
		if ctr.State != StateRunning {
			out.ReclaimableBytes += size
		}
	}
	for _, vol := range du.Volumes {
		if vol == nil || vol.UsageData == nil || vol.UsageData.Size <= 0 {
			continue
		}
		size := uint64(vol.UsageData.Size)
		out.VolumesBytes += size
		if vol.UsageData.RefCount == 0 {
			out.ReclaimableBytes += size
		}
	}
	for _, bc := range du.BuildCache {
		if bc == nil || bc.Size <= 0 {
			continue
		}
		size := uint64(bc.Size)
		out.BuildCacheBytes += size
		if !bc.InUse {
			out.ReclaimableBytes += size
		}
	}

	out.TotalBytes = out.ImagesBytes + out.ContainersBytes + out.VolumesBytes + out.BuildCacheBytes
	return out, nil
}

// CollectTelemetry gathers capacity, disk usage, and live container stats in one snapshot.
func CollectTelemetry(ctx context.Context, cli StatsCollector) (Telemetry, error) {
	capacity, err := CollectCapacity(ctx, cli)
	if err != nil {
		return Telemetry{}, err
	}
	disk, err := CollectDiskUsage(ctx, cli)
	if err != nil {
		return Telemetry{}, err
	}
	containers, err := CollectContainerStats(ctx, cli)
	if err != nil {
		return Telemetry{}, err
	}

	var totalCPU float64
	var totalMem uint64
	for _, st := range containers {
		totalCPU += st.CPUPct
		totalMem += st.MemUsedBytes
	}

	return Telemetry{
		Capacity:      capacity,
		DiskUsage:     disk,
		Containers:    containers,
		TotalCPUPct:   totalCPU,
		TotalMemBytes: totalMem,
	}, nil
}
