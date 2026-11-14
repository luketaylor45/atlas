package installer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type ProgressUpdate struct {
	Stage    string
	Progress int
}

type ProgressReporter func(ProgressUpdate)

type lineTailWriter struct {
	lines   []string
	partial string
	limit   int
}

func newLineTailWriter(limit int) *lineTailWriter {
	return &lineTailWriter{limit: max(limit, 1)}
}

func (w *lineTailWriter) Write(data []byte) (int, error) {
	length := len(data)
	parts := strings.Split(w.partial+string(data), "\n")
	w.partial = parts[len(parts)-1]
	for _, part := range parts[:len(parts)-1] {
		w.append(part)
	}
	return length, nil
}

func (w *lineTailWriter) String() string {
	lines := append([]string(nil), w.lines...)
	if partial := strings.TrimSpace(w.partial); partial != "" {
		lines = append(lines, partial)
	}
	if len(lines) > w.limit {
		lines = lines[len(lines)-w.limit:]
	}
	return strings.Join(lines, " | ")
}

func (w *lineTailWriter) append(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	w.lines = append(w.lines, line)
	if len(w.lines) > w.limit {
		w.lines = w.lines[len(w.lines)-w.limit:]
	}
}

var (
	steamProgressPattern = regexp.MustCompile(`(?i)progress:\s*([0-9]+(?:\.[0-9]+)?)`)
	steamStatePattern    = regexp.MustCompile(`(?i)update state\s*(?:\([^)]*\))?\s*([^,]+)`)
	steamPhasePattern    = regexp.MustCompile(`(?i)^steamcmd phase\s+([0-9]+)\s*/\s*([0-9]+)\s*:\s*(.+)$`)
	percentPattern       = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*%`)
	curlProgressPattern  = regexp.MustCompile(`^\s*([0-9]{1,3})\s+\S+\s+[0-9]{1,3}\s+`)
)

type progressTracker struct {
	reporter     ProgressReporter
	lastStage    string
	lastProgress int
	lastRaw      int
	steamStart   int
	steamEnd     int
	steamLabel   string
}

func newProgressTracker(reporter ProgressReporter, initialProgress int) *progressTracker {
	return &progressTracker{
		reporter:     reporter,
		lastProgress: initialProgress,
		lastRaw:      -1,
		steamStart:   10,
		steamEnd:     95,
	}
}

func (t *progressTracker) observe(line string) {
	if matches := steamProgressPattern.FindStringSubmatch(line); len(matches) == 2 {
		raw, ok := parsePercent(matches[1])
		if !ok || raw == t.lastRaw {
			return
		}

		phase := "working"
		if state := steamStatePattern.FindStringSubmatch(line); len(state) == 2 {
			phase = strings.ToLower(strings.TrimSpace(state[1]))
		}

		stage := fmt.Sprintf("SteamCMD %s: %d%%", phase, raw)
		if t.steamLabel != "" {
			stage = fmt.Sprintf("%s (%s): %d%%", t.steamLabel, phase, raw)
		}

		t.lastRaw = raw
		t.emit(stage, t.scaleSteamProgress(raw))
		return
	}

	if matches := percentPattern.FindStringSubmatch(line); len(matches) == 2 {
		raw, ok := parsePercent(matches[1])
		if !ok || raw == t.lastRaw {
			return
		}

		stage := fmt.Sprintf("Downloading files: %d%%", raw)
		if t.steamLabel != "" {
			stage = fmt.Sprintf("%s: %d%%", t.steamLabel, raw)
		}

		t.lastRaw = raw
		t.emit(stage, t.scaleSteamProgress(raw))
		return
	}

	if matches := curlProgressPattern.FindStringSubmatch(line); len(matches) == 2 {
		raw, ok := parsePercent(matches[1])
		if !ok || raw == t.lastRaw {
			return
		}

		t.lastRaw = raw
		t.emit(fmt.Sprintf("Downloading files: %d%%", raw), 10+(raw*85/100))
		return
	}

	marker := strings.Index(line, "[Atlas]")
	if marker < 0 {
		return
	}

	stage := strings.TrimSpace(line[marker+len("[Atlas]"):])
	if stage == "" {
		return
	}

	if matches := steamPhasePattern.FindStringSubmatch(stage); len(matches) == 4 {
		current, currentErr := strconv.Atoi(matches[1])
		total, totalErr := strconv.Atoi(matches[2])
		if currentErr == nil && totalErr == nil && total > 0 && current > 0 && current <= total {
			t.steamStart = 10 + ((current - 1) * 85 / total)
			t.steamEnd = 10 + (current * 85 / total)
			t.steamLabel = strings.TrimSpace(matches[3])
			t.lastRaw = -1
			t.emit(t.steamLabel, t.steamStart)
			return
		}
	}

	progress := t.lastProgress
	lowerStage := strings.ToLower(stage)
	switch {
	case strings.Contains(lowerStage, "warming"), strings.Contains(lowerStage, "bootstrap"):
		progress = max(progress, 5)
	case strings.Contains(lowerStage, "install"), strings.Contains(lowerStage, "download"):
		progress = max(progress, 10)
	case strings.Contains(lowerStage, "setting"), strings.Contains(lowerStage, "final"):
		progress = max(progress, 96)
	}

	t.emit(stage, progress)
}

func (t *progressTracker) scaleSteamProgress(raw int) int {
	return t.steamStart + (raw * (t.steamEnd - t.steamStart) / 100)
}

func (t *progressTracker) emit(stage string, progress int) {
	progress = min(max(progress, t.lastProgress), 100)
	if stage == t.lastStage && progress == t.lastProgress {
		return
	}

	t.lastStage = stage
	t.lastProgress = progress
	if t.reporter != nil {
		t.reporter(ProgressUpdate{Stage: stage, Progress: progress})
	}
}

func parsePercent(value string) (int, bool) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return min(max(int(math.Round(parsed)), 0), 100), true
}

func splitOutputLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for index, char := range data {
		if char != '\n' && char != '\r' {
			continue
		}

		advance = index + 1
		if char == '\r' && len(data) > advance && data[advance] == '\n' {
			advance++
		}
		return advance, data[:index], nil
	}

	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func streamScriptOutput(reader io.Reader, output io.Writer, reporter ProgressReporter) error {
	tracker := newProgressTracker(reporter, 10)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	scanner.Split(splitOutputLines)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fmt.Fprintln(output, line)
		tracker.observe(line)
	}
	return scanner.Err()
}

type imagePullMessage struct {
	ID             string `json:"id"`
	Error          string `json:"error"`
	ProgressDetail struct {
		Current int64 `json:"current"`
		Total   int64 `json:"total"`
	} `json:"progressDetail"`
}

type imagePullLayer struct {
	current int64
	total   int64
}

// ConsumeImagePull drains Docker's JSON stream and reports aggregate layer progress.
func ConsumeImagePull(reader io.Reader, report func(int)) error {
	decoder := json.NewDecoder(reader)
	layers := make(map[string]imagePullLayer)
	lastProgress := -1

	for {
		var message imagePullMessage
		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("decode image pull progress: %w", err)
		}
		if message.Error != "" {
			return fmt.Errorf("image pull failed: %s", message.Error)
		}
		if message.ID == "" || message.ProgressDetail.Total <= 0 {
			continue
		}

		layers[message.ID] = imagePullLayer{
			current: min(message.ProgressDetail.Current, message.ProgressDetail.Total),
			total:   message.ProgressDetail.Total,
		}

		var current, total int64
		for _, layer := range layers {
			current += layer.current
			total += layer.total
		}
		if total == 0 {
			continue
		}

		progress := int(math.Round(float64(current) * 100 / float64(total)))
		progress = min(max(progress, lastProgress), 99)
		if progress != lastProgress {
			lastProgress = progress
			if report != nil {
				report(progress)
			}
		}
	}

	if report != nil && lastProgress < 100 {
		report(100)
	}
	return nil
}
