package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/luketaylor45/atlas/daemon/internal/config"
)

var (
	errInstallCancelled = errors.New("installation was cancelled")
	errInstallTimedOut  = errors.New("installation timed out")
)

// installRegistry guarantees at most one provisioning job per service and
// lets a running job be cancelled.
type installRegistry struct {
	mu   sync.Mutex
	jobs map[string]*installJob
}

type installJob struct {
	cancel    context.CancelCauseFunc
	startedAt time.Time
}

var installs = &installRegistry{jobs: make(map[string]*installJob)}

// begin reserves the service for one job. It returns false when a job is
// already running. The returned finish function must be called when the job ends.
func (r *installRegistry) begin(uuid string, timeout time.Duration) (context.Context, func(), bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, running := r.jobs[uuid]; running {
		return nil, nil, false
	}

	ctx, cancelCause := context.WithCancelCause(context.Background())
	ctx, cancelTimeout := context.WithTimeoutCause(ctx, timeout, errInstallTimedOut)
	job := &installJob{cancel: cancelCause, startedAt: time.Now()}
	r.jobs[uuid] = job

	finish := func() {
		cancelTimeout()
		cancelCause(nil)
		r.mu.Lock()
		if r.jobs[uuid] == job {
			delete(r.jobs, uuid)
		}
		r.mu.Unlock()
	}
	return ctx, finish, true
}

func (r *installRegistry) cancel(uuid string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, running := r.jobs[uuid]
	if running {
		job.cancel(errInstallCancelled)
	}
	return running
}

func (r *installRegistry) active(uuid string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, running := r.jobs[uuid]
	return running
}

func (r *installRegistry) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	uuids := make([]string, 0, len(r.jobs))
	for uuid := range r.jobs {
		uuids = append(uuids, uuid)
	}
	sort.Strings(uuids)
	return uuids
}

func installTimeout() time.Duration {
	minutes := config.NodeConfig.InstallTimeoutMinutes
	if minutes <= 0 {
		minutes = 120
	}
	return time.Duration(minutes) * time.Minute
}

// Atlas keeps its own working files beside, not inside, the service
// directories so a reinstall never wipes them and users never see them.
func atlasDir(parts ...string) string {
	return filepath.Join(append([]string{config.NodeConfig.DataPath, ".atlas"}, parts...)...)
}

func atlasHostDir(parts ...string) string {
	return filepath.Join(append([]string{config.NodeConfig.HostDataPath, ".atlas"}, parts...)...)
}

func installLogPath(uuid string) string {
	return atlasDir("logs", uuid+".log")
}

// installLog is the full installer transcript for one service.
type installLog struct {
	mu   sync.Mutex
	file *os.File
}

func openInstallLog(uuid string) (*installLog, error) {
	path := installLogPath(uuid)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create install log directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("open install log: %w", err)
	}
	return &installLog{file: file}, nil
}

func (l *installLog) Write(data []byte) (int, error) {
	if l == nil {
		return len(data), nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Write(data)
}

// Printf adds an Atlas status line to the transcript.
func (l *installLog) Printf(format string, args ...any) {
	if l == nil {
		return
	}
	fmt.Fprintf(l, "[Atlas] "+format+"\n", args...)
}

func (l *installLog) Close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_ = l.file.Close()
}

const maxInstallLogChunk = 256 * 1024

// readInstallLog returns log content starting at offset and the offset to ask
// for next. An offset beyond the file (the log was restarted) reads from the
// beginning; a very long backlog is cut to its most recent part.
func readInstallLog(uuid string, offset int64) (string, int64, error) {
	file, err := os.Open(installLogPath(uuid))
	if errors.Is(err, os.ErrNotExist) {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", 0, err
	}
	size := info.Size()
	if offset < 0 || offset > size {
		offset = 0
	}
	if size-offset > maxInstallLogChunk {
		offset = size - maxInstallLogChunk
	}

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return "", 0, err
	}
	data, err := io.ReadAll(io.LimitReader(file, maxInstallLogChunk))
	if err != nil {
		return "", 0, err
	}
	return string(data), offset + int64(len(data)), nil
}
