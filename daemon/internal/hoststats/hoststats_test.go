package hoststats

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestSamplerReportsUsageBetweenSamples(t *testing.T) {
	dir := t.TempDir()
	sampler := &Sampler{
		statPath:    filepath.Join(dir, "stat"),
		meminfoPath: filepath.Join(dir, "meminfo"),
	}
	write(t, sampler.meminfoPath, "MemTotal:       16384000 kB\nMemFree:         1000000 kB\nMemAvailable:    8192000 kB\n")

	// user nice system idle iowait irq softirq steal guest guest_nice
	write(t, sampler.statPath, "cpu  1000 0 1000 8000 0 0 0 0 0 0\ncpu0 500 0 500 4000 0 0 0 0 0 0\n")
	first := sampler.Sample()
	if first.CPUPercent != 0 {
		t.Fatalf("first sample CPU = %v, want 0", first.CPUPercent)
	}
	if math.Abs(first.UsedRAMMB-8000) > 0.01 || math.Abs(first.TotalRAMMB-16000) > 0.01 {
		t.Fatalf("memory = %v used of %v, want 8000 of 16000", first.UsedRAMMB, first.TotalRAMMB)
	}

	// 1000 more ticks: 250 busy, 750 idle.
	write(t, sampler.statPath, "cpu  1200 0 1050 8700 50 0 0 0 0 0\n")
	second := sampler.Sample()
	if math.Abs(second.CPUPercent-25) > 0.01 {
		t.Fatalf("second sample CPU = %v, want 25", second.CPUPercent)
	}
}

func TestSamplerReportsZerosWithoutProc(t *testing.T) {
	sampler := &Sampler{statPath: "/does/not/exist", meminfoPath: "/does/not/exist"}
	if snapshot := sampler.Sample(); snapshot != (Snapshot{}) {
		t.Fatalf("snapshot = %+v, want zeros", snapshot)
	}
}
