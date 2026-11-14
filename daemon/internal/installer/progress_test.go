package installer

import (
	"bytes"
	"io"
	"slices"
	"strings"
	"testing"
)

func TestProgressTrackerParsesSteamCMDAndKeepsGlobalProgressMonotonic(t *testing.T) {
	var updates []ProgressUpdate
	tracker := newProgressTracker(func(update ProgressUpdate) {
		updates = append(updates, update)
	}, 10)

	tracker.observe("Update state (0x61) downloading, progress: 42.37 (100 / 200)")
	tracker.observe("Update state (0x61) downloading, progress: 42.49 (101 / 200)")
	tracker.observe("Update state (0x81) verifying update, progress: 3.00 (3 / 100)")

	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d: %#v", len(updates), updates)
	}
	if updates[0].Stage != "SteamCMD downloading: 42%" || updates[0].Progress != 45 {
		t.Fatalf("unexpected download update: %#v", updates[0])
	}
	if updates[1].Stage != "SteamCMD verifying update: 3%" {
		t.Fatalf("unexpected verify stage: %#v", updates[1])
	}
	if updates[1].Progress != updates[0].Progress {
		t.Fatalf("global progress regressed from %d to %d", updates[0].Progress, updates[1].Progress)
	}
}

func TestProgressTrackerParsesCurlMeter(t *testing.T) {
	var updates []ProgressUpdate
	tracker := newProgressTracker(func(update ProgressUpdate) {
		updates = append(updates, update)
	}, 10)

	tracker.observe(" 37  100M   37 37.0M    0     0  1024k      0  0:01:40  0:00:37  0:01:03 1024k")

	if len(updates) != 1 {
		t.Fatalf("expected a curl progress update, got %#v", updates)
	}
	if updates[0].Stage != "Downloading files: 37%" {
		t.Fatalf("unexpected curl stage: %q", updates[0].Stage)
	}
}

func TestProgressTrackerPartitionsSteamCMDPhases(t *testing.T) {
	var updates []ProgressUpdate
	tracker := newProgressTracker(func(update ProgressUpdate) {
		updates = append(updates, update)
	}, 10)

	tracker.observe("[Atlas] SteamCMD phase 1/3: Updating SteamCMD")
	tracker.observe("Update state (0x61) downloading, progress: 100.00")
	tracker.observe("[Atlas] SteamCMD phase 2/3: Downloading Windows files")
	tracker.observe("Update state (0x61) downloading, progress: 50.00")
	tracker.observe("[Atlas] SteamCMD phase 3/3: Downloading Linux files")
	tracker.observe("Update state (0x61) downloading, progress: 50.00")

	if got, want := updates[len(updates)-1].Progress, 80; got != want {
		t.Fatalf("final progress = %d, want %d; updates: %#v", got, want, updates)
	}
	if got, want := updates[len(updates)-1].Stage, "Downloading Linux files (downloading): 50%"; got != want {
		t.Fatalf("final stage = %q, want %q", got, want)
	}
	if !slices.IsSortedFunc(updates, func(a, b ProgressUpdate) int {
		return a.Progress - b.Progress
	}) {
		t.Fatalf("expected monotonic phased progress, got %#v", updates)
	}
}

func TestStreamScriptOutputSplitsCarriageReturnProgress(t *testing.T) {
	input := "[Atlas] Bootstrap\rUpdate state (0x61) downloading, progress: 50.00\r\n"
	var output bytes.Buffer
	var updates []ProgressUpdate

	err := streamScriptOutput(strings.NewReader(input), &output, func(update ProgressUpdate) {
		updates = append(updates, update)
	})
	if err != nil {
		t.Fatalf("streamScriptOutput returned an error: %v", err)
	}
	if !strings.Contains(output.String(), "Bootstrap\n") || !strings.Contains(output.String(), "progress: 50.00\n") {
		t.Fatalf("unexpected normalized output: %q", output.String())
	}
	if len(updates) != 2 {
		t.Fatalf("expected two progress updates, got %#v", updates)
	}
}

func TestConsumeImagePullReportsMonotonicProgressAndCompletion(t *testing.T) {
	stream := strings.Join([]string{
		`{"id":"layer-a","progressDetail":{"current":25,"total":100}}`,
		`{"id":"layer-b","progressDetail":{"current":50,"total":100}}`,
		`{"id":"layer-a","progressDetail":{"current":100,"total":100}}`,
	}, "\n")

	var progress []int
	if err := ConsumeImagePull(strings.NewReader(stream), func(value int) {
		progress = append(progress, value)
	}); err != nil {
		t.Fatalf("ConsumeImagePull returned an error: %v", err)
	}

	if len(progress) == 0 || progress[len(progress)-1] != 100 {
		t.Fatalf("expected completion at 100, got %v", progress)
	}
	if !slices.IsSorted(progress) {
		t.Fatalf("expected monotonic progress, got %v", progress)
	}
}

func TestConsumeImagePullReturnsStreamError(t *testing.T) {
	err := ConsumeImagePull(strings.NewReader(`{"error":"unauthorized"}`), nil)
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("expected Docker stream error, got %v", err)
	}
}

func TestLineTailWriterKeepsRecentCompleteLines(t *testing.T) {
	tail := newLineTailWriter(3)
	_, _ = tail.Write([]byte("one\ntwo\nthree\n"))
	_, _ = tail.Write([]byte("four\npartial"))

	if got, want := tail.String(), "three | four | partial"; got != want {
		t.Fatalf("tail = %q, want %q", got, want)
	}
}

func BenchmarkStreamScriptOutput(b *testing.B) {
	line := strings.Repeat("Update state (0x61) downloading, progress: 42.00\r", 100)
	for b.Loop() {
		if err := streamScriptOutput(strings.NewReader(line), io.Discard, nil); err != nil {
			b.Fatal(err)
		}
	}
}
