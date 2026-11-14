package api

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/daemon/internal/docker"
	"github.com/luketaylor45/atlas/daemon/internal/serverfs"
)

// backupTimeout bounds one backup or restore so a stuck disk cannot hold a
// server's backup slot forever.
const backupTimeout = 6 * time.Hour

var errBackupCancelled = errors.New("the backup was cancelled")

// backupRegistry tracks running backup and restore jobs by backup ID.
type backupRegistry struct {
	mu   sync.Mutex
	jobs map[string]*backupJob
}

type backupJob struct {
	service string
	restore bool
	cancel  context.CancelCauseFunc
	// release frees the job's timer once it has ended.
	release context.CancelFunc
	done    chan struct{}
}

var backups = &backupRegistry{jobs: make(map[string]*backupJob)}

// begin registers a job. It returns false when the backup already has one, or
// when the service is being restored (nothing else may touch its files then).
func (r *backupRegistry) begin(id string, service string, restore bool) (context.Context, *backupJob, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, running := r.jobs[id]; running {
		return nil, nil, false
	}
	for _, job := range r.jobs {
		if job.service == service && (job.restore || restore) {
			return nil, nil, false
		}
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	ctx, release := context.WithTimeoutCause(ctx, backupTimeout, errors.New("the job exceeded its time limit"))
	job := &backupJob{service: service, restore: restore, cancel: cancel, release: release, done: make(chan struct{})}
	r.jobs[id] = job
	return ctx, job, true
}

func (r *backupRegistry) finish(id string, job *backupJob) {
	job.release()
	job.cancel(nil)
	r.mu.Lock()
	if r.jobs[id] == job {
		delete(r.jobs, id)
	}
	r.mu.Unlock()
	close(job.done)
}

// cancel stops a backup's job and waits briefly for it to let go of its files.
func (r *backupRegistry) cancel(id string) {
	r.mu.Lock()
	job, running := r.jobs[id]
	r.mu.Unlock()
	if !running {
		return
	}
	job.cancel(errBackupCancelled)
	select {
	case <-job.done:
	case <-time.After(30 * time.Second):
	}
}

// cancelService stops every job of a service that is being deleted.
func (r *backupRegistry) cancelService(service string) {
	r.mu.Lock()
	var ids []string
	for id, job := range r.jobs {
		if job.service == service {
			ids = append(ids, id)
		}
	}
	r.mu.Unlock()
	for _, id := range ids {
		r.cancel(id)
	}
}

// restoring reports whether a service's files are being replaced right now.
func (r *backupRegistry) restoring(service string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, job := range r.jobs {
		if job.service == service && job.restore {
			return true
		}
	}
	return false
}

// list returns the IDs of backups with a job in progress.
func (r *backupRegistry) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := make([]string, 0, len(r.jobs))
	for id := range r.jobs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func backupsDir(service string) string {
	return atlasDir("backups", service)
}

func backupPath(service string, id string) string {
	return filepath.Join(backupsDir(service), id+".tar.gz")
}

// backupID reads and checks the backup identifier in the route.
func backupID(c *gin.Context) (string, bool) {
	id := c.Param("backup")
	if !serverfs.IsServiceDirectoryName(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup identifier"})
		return "", false
	}
	return id, true
}

// notifyBackup reports the end of a job to Core. The report matters (without
// it a finished backup looks interrupted), so it is retried.
func notifyBackup(id string, operation string, payload map[string]any) {
	payload["operation"] = operation
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 5 * time.Second)
		}
		if err = postCoreJSON("/api/v1/internal/backups/"+id+"/status", payload); err == nil {
			return
		}
	}
	log.Printf("[Daemon] Could not report the %s of backup %s to Core: %v", operation, id, err)
}

func jobError(ctx context.Context, err error) error {
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	return err
}

// HandleCreateBackup starts archiving a service's files.
func HandleCreateBackup(c *gin.Context) {
	service := c.Param("uuid")
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !serverfs.IsServiceDirectoryName(req.ID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup identifier"})
		return
	}
	if _, err := os.Stat(serverfs.LocalServicePath(service)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "This server has no files on the node"})
		return
	}
	if installs.active(service) {
		c.JSON(http.StatusConflict, gin.H{"error": "This server is being installed. Wait for the installation to finish."})
		return
	}

	ctx, job, ok := backups.begin(req.ID, service, false)
	if !ok {
		c.JSON(http.StatusConflict, gin.H{"error": "A backup is already being restored or created for this server"})
		return
	}

	go func() {
		// The job stays listed until Core has been told, so Core never
		// sees "no job" for a backup that is about to be reported.
		defer backups.finish(req.ID, job)

		size, checksum, err := writeBackup(ctx, service, req.ID)
		if err != nil {
			err = jobError(ctx, err)
			log.Printf("[Daemon] Backup %s of %s failed: %v", req.ID, service, err)
			_ = os.Remove(backupPath(service, req.ID))
			if errors.Is(err, errBackupCancelled) {
				return
			}
			notifyBackup(req.ID, "create", map[string]any{"status": "failed", "error": err.Error()})
			return
		}
		log.Printf("[Daemon] Backup %s of %s finished (%d bytes)", req.ID, service, size)
		notifyBackup(req.ID, "create", map[string]any{"status": "completed", "size": size, "checksum": "sha256:" + checksum})
	}()

	c.JSON(http.StatusAccepted, gin.H{"status": "creating"})
}

// writeBackup archives a service directory into a gzip-compressed tar file
// and returns its size and SHA-256.
func writeBackup(ctx context.Context, service string, id string) (int64, string, error) {
	source := serverfs.LocalServicePath(service)
	target := backupPath(service, id)
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return 0, "", fmt.Errorf("prepare backup storage: %w", err)
	}

	// Written under a temporary name so a half-written archive is never
	// mistaken for a backup.
	temporary := target + ".part"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return 0, "", fmt.Errorf("create backup file: %w", err)
	}
	defer os.Remove(temporary)

	hasher := sha256.New()
	counter := &countingWriter{}
	compressor := gzip.NewWriter(io.MultiWriter(file, hasher, counter))
	archive := tar.NewWriter(compressor)

	walkErr := filepath.WalkDir(source, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			// A file deleted by the running game mid-backup is not a failure.
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(source, current)
		if err != nil || relative == "." {
			return err
		}
		return archiveEntry(archive, current, filepath.ToSlash(relative), entry)
	})

	closeErr := errors.Join(archive.Close(), compressor.Close(), file.Close())
	if walkErr != nil {
		return 0, "", walkErr
	}
	if closeErr != nil {
		return 0, "", fmt.Errorf("finish backup file: %w", closeErr)
	}
	if err := os.Rename(temporary, target); err != nil {
		return 0, "", fmt.Errorf("store backup file: %w", err)
	}
	return counter.count, hex.EncodeToString(hasher.Sum(nil)), nil
}

type countingWriter struct{ count int64 }

func (w *countingWriter) Write(data []byte) (int, error) {
	w.count += int64(len(data))
	return len(data), nil
}

// archiveEntry adds one file, directory or symbolic link to the archive.
// Anything else (sockets, devices) is left out.
func archiveEntry(archive *tar.Writer, current string, name string, entry fs.DirEntry) error {
	info, err := entry.Info()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	link := ""
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		if link, err = os.Readlink(current); err != nil {
			return nil
		}
	case info.IsDir(), info.Mode().IsRegular():
	default:
		return nil
	}

	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	header.Name = name
	if info.IsDir() {
		header.Name += "/"
	}
	// Ownership is set again on restore, so the node's own IDs are not recorded.
	header.Uid, header.Gid, header.Uname, header.Gname = 0, 0, "", ""

	if !info.Mode().IsRegular() {
		return archive.WriteHeader(header)
	}

	file, err := os.Open(current)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read %s: %w", name, err)
	}
	defer file.Close()

	if err := archive.WriteHeader(header); err != nil {
		return err
	}
	// A running game may change a file while it is read. The archive needs
	// exactly the size its header promised: extra bytes are left out and a
	// file that shrank is padded.
	written, err := io.CopyN(archive, file, header.Size)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read %s: %w", name, err)
	}
	if written < header.Size {
		if _, err := io.CopyN(archive, zeroReader{}, header.Size-written); err != nil {
			return err
		}
	}
	return nil
}

type zeroReader struct{}

func (zeroReader) Read(data []byte) (int, error) {
	clear(data)
	return len(data), nil
}

// HandleDeleteBackup removes a backup archive. Deleting one that is not
// there succeeds, so Core can always clean up its record.
func HandleDeleteBackup(c *gin.Context) {
	service := c.Param("uuid")
	id, ok := backupID(c)
	if !ok {
		return
	}

	backups.cancel(id)
	if err := os.Remove(backupPath(service, id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete the backup: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// HandleDownloadBackup streams a backup archive to Core.
func HandleDownloadBackup(c *gin.Context) {
	service := c.Param("uuid")
	id, ok := backupID(c)
	if !ok {
		return
	}
	archive := backupPath(service, id)
	if _, err := os.Stat(archive); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "This backup's file is missing on the node"})
		return
	}
	c.FileAttachment(archive, id+".tar.gz")
}

// HandleRestoreBackup replaces a stopped server's files with a backup's.
func HandleRestoreBackup(c *gin.Context) {
	service := c.Param("uuid")
	id, ok := backupID(c)
	if !ok {
		return
	}
	var req struct {
		Truncate bool `json:"truncate"`
	}
	_ = c.ShouldBindJSON(&req)

	if _, err := os.Stat(backupPath(service, id)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "This backup's file is missing on the node"})
		return
	}
	if installs.active(service) {
		c.JSON(http.StatusConflict, gin.H{"error": "This server is being installed. Wait for the installation to finish."})
		return
	}
	if inspect, err := docker.Client.ContainerInspect(c.Request.Context(), service); err == nil && inspect.State != nil && inspect.State.Running {
		c.JSON(http.StatusConflict, gin.H{"error": "Stop the server before restoring a backup"})
		return
	}

	ctx, job, ok := backups.begin(id, service, true)
	if !ok {
		c.JSON(http.StatusConflict, gin.H{"error": "Another backup job is running for this server. Try again when it has finished."})
		return
	}

	go func() {
		defer backups.finish(id, job)

		err := restoreBackup(ctx, service, id, req.Truncate)
		if err != nil {
			err = jobError(ctx, err)
			log.Printf("[Daemon] Restore of backup %s to %s failed: %v", id, service, err)
			notifyBackup(id, "restore", map[string]any{"status": "failed", "error": err.Error()})
			return
		}
		log.Printf("[Daemon] Restored backup %s to %s", id, service)
		notifyBackup(id, "restore", map[string]any{"status": "completed"})
	}()

	c.JSON(http.StatusAccepted, gin.H{"status": "restoring"})
}

// restoreBackup extracts a backup over a service directory. With truncate
// the directory is emptied first, so nothing outside the backup survives.
func restoreBackup(ctx context.Context, service string, id string, truncate bool) error {
	dataDir := serverfs.LocalServicePath(service)
	if err := serverfs.EnsureServiceDirectory(dataDir); err != nil {
		return fmt.Errorf("prepare server directory: %w", err)
	}
	// Whatever happens, the files end up owned by the game's user.
	defer func() {
		if err := serverfs.NormalizeTree(dataDir); err != nil {
			log.Printf("[Daemon] Could not normalize permissions after restoring %s: %v", service, err)
		}
	}()

	_, installedErr := os.Stat(filepath.Join(dataDir, installedMarker))
	wasInstalled := installedErr == nil

	if truncate {
		if err := wipeServiceFiles(dataDir); err != nil {
			return err
		}
	}

	file, err := os.Open(backupPath(service, id))
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer file.Close()
	decompressor, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}
	defer decompressor.Close()

	// Everything is written through a root handle, which refuses any path
	// (or symbolic link) that would lead outside the server directory.
	root, err := os.OpenRoot(dataDir)
	if err != nil {
		return fmt.Errorf("open server directory: %w", err)
	}
	defer root.Close()

	archive := tar.NewReader(decompressor)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read backup: %w", err)
		}
		if err := restoreEntry(root, archive, header); err != nil {
			return err
		}
	}

	// A server that was installed stays installed, even if the backup
	// predates the marker or the directory was emptied.
	if wasInstalled {
		marker := filepath.Join(dataDir, installedMarker)
		if _, err := os.Stat(marker); err != nil {
			_ = serverfs.WriteFile(marker, []byte(time.Now().Format(time.RFC3339)), 0660)
		}
	}
	_ = os.Remove(filepath.Join(dataDir, installFailedMarker))
	return nil
}

func restoreEntry(root *os.Root, archive *tar.Reader, header *tar.Header) error {
	name := path.Clean(strings.TrimPrefix(header.Name, "/"))
	if name == "." || name == ".." || strings.HasPrefix(name, "../") {
		return nil
	}
	name = filepath.FromSlash(name)
	mode := os.FileMode(header.Mode).Perm()

	switch header.Typeflag {
	case tar.TypeDir:
		if err := root.MkdirAll(name, mode|0770); err != nil {
			return fmt.Errorf("restore %s: %w", header.Name, err)
		}

	case tar.TypeReg:
		if parent := filepath.Dir(name); parent != "." {
			if err := root.MkdirAll(parent, 0770); err != nil {
				return fmt.Errorf("restore %s: %w", header.Name, err)
			}
		}
		// Whatever is in the way (an old file, a link, a directory) is replaced.
		if err := root.RemoveAll(name); err != nil {
			return fmt.Errorf("restore %s: %w", header.Name, err)
		}
		file, err := root.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_EXCL, mode|0660)
		if err != nil {
			return fmt.Errorf("restore %s: %w", header.Name, err)
		}
		_, copyErr := io.Copy(file, archive)
		if err := errors.Join(copyErr, file.Close()); err != nil {
			return fmt.Errorf("restore %s: %w", header.Name, err)
		}
		_ = root.Chtimes(name, time.Now(), header.ModTime)

	case tar.TypeSymlink:
		if parent := filepath.Dir(name); parent != "." {
			if err := root.MkdirAll(parent, 0770); err != nil {
				return fmt.Errorf("restore %s: %w", header.Name, err)
			}
		}
		if err := root.RemoveAll(name); err != nil {
			return fmt.Errorf("restore %s: %w", header.Name, err)
		}
		if err := root.Symlink(header.Linkname, name); err != nil {
			return fmt.Errorf("restore %s: %w", header.Name, err)
		}
	}
	// Other entry types are never written by Atlas and are ignored.
	return nil
}
