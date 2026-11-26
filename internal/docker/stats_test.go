package docker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/api/types/volume"
)

type mockStatsCollector struct {
	running   []container.Summary
	statsJSON map[string]string
	statsErr  map[string]error
	info      system.Info
	du        types.DiskUsage
}

func (m *mockStatsCollector) ContainerList(_ context.Context, _ container.ListOptions) ([]container.Summary, error) {
	return m.running, nil
}

func (m *mockStatsCollector) ContainerStatsOneShot(_ context.Context, id string) (container.StatsResponseReader, error) {
	if err, ok := m.statsErr[id]; ok {
		return container.StatsResponseReader{}, err
	}
	raw, ok := m.statsJSON[id]
	if !ok {
		return container.StatsResponseReader{}, errors.New("missing")
	}
	return container.StatsResponseReader{
		Body: io.NopCloser(strings.NewReader(raw)),
	}, nil
}

func (m *mockStatsCollector) Info(_ context.Context) (system.Info, error) {
	return m.info, nil
}

func (m *mockStatsCollector) DiskUsage(_ context.Context, _ types.DiskUsageOptions) (types.DiskUsage, error) {
	return m.du, nil
}

func TestCalculateCPUAndMemZeroSafe(t *testing.T) {
	if got := CalculateCPUPercent(container.StatsResponse{}); got != 0 {
		t.Fatalf("expected 0 CPU for empty stats, got %v", got)
	}

	used, limit, pct := CalculateMemUsage(container.MemoryStats{
		Usage: 1024,
		Limit: 0,
		Stats: map[string]uint64{"inactive_file": 256},
	})
	if used != 768 || limit != 0 || pct != 0 {
		t.Fatalf("unexpected mem result: used=%d limit=%d pct=%v", used, limit, pct)
	}
}

func TestCollectTelemetry(t *testing.T) {
	statPayload, err := json.Marshal(container.StatsResponse{
		Name: "/redis",
		CPUStats: container.CPUStats{
			CPUUsage:    container.CPUUsage{TotalUsage: 200},
			SystemUsage: 1000,
			OnlineCPUs:  2,
		},
		PreCPUStats: container.CPUStats{
			CPUUsage:    container.CPUUsage{TotalUsage: 100},
			SystemUsage: 500,
		},
		MemoryStats: container.MemoryStats{
			Usage: 600,
			Limit: 1000,
			Stats: map[string]uint64{"cache": 100},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	mock := &mockStatsCollector{
		running: []container.Summary{
			{ID: "c1", Names: []string{"/redis"}},
			{ID: "c2", Names: []string{"/stopped-midway"}},
		},
		statsJSON: map[string]string{
			"c1": string(statPayload),
		},
		statsErr: map[string]error{
			"c2": errors.New("container stopped"),
		},
		info: system.Info{
			NCPU:     8,
			MemTotal: 16 * 1024 * 1024 * 1024,
		},
		du: types.DiskUsage{
			Images: []*image.Summary{
				{Size: 1000, Containers: 1},
				{Size: 500, Containers: 0},
			},
			Containers: []*container.Summary{
				{SizeRw: 200, State: StateRunning},
				{SizeRw: 100, State: "exited"},
			},
			Volumes: []*volume.Volume{
				{UsageData: &volume.UsageData{Size: 300, RefCount: 0}},
			},
			BuildCache: []*build.CacheRecord{
				{Size: 400, InUse: false},
			},
		},
	}

	tel, err := CollectTelemetry(context.Background(), mock)
	if err != nil {
		t.Fatalf("CollectTelemetry() error = %v", err)
	}
	if tel.Capacity.NCPU != 8 || tel.Capacity.MemTotalBytes != 16*1024*1024*1024 {
		t.Fatalf("unexpected capacity: %+v", tel.Capacity)
	}
	if tel.DiskUsage.ImagesBytes != 1500 || tel.DiskUsage.ContainersBytes != 300 ||
		tel.DiskUsage.VolumesBytes != 300 || tel.DiskUsage.BuildCacheBytes != 400 ||
		tel.DiskUsage.TotalBytes != 2500 || tel.DiskUsage.ReclaimableBytes != 1300 {
		t.Fatalf("unexpected disk usage: %+v", tel.DiskUsage)
	}

	redisStat, ok := tel.Containers["redis"]
	if !ok {
		t.Fatal("expected redis stat")
	}
	if math.Abs(redisStat.CPUPct-40.0) > 1e-6 {
		t.Fatalf("redis CPUPct = %v, want 40.0", redisStat.CPUPct)
	}
	if redisStat.MemUsedBytes != 500 || math.Abs(redisStat.MemPct-50.0) > 1e-6 {
		t.Fatalf("unexpected redis mem: %+v", redisStat)
	}
	if len(tel.Containers) != 1 {
		t.Fatalf("expected stopped-midway container to be skipped, got %d", len(tel.Containers))
	}
}
