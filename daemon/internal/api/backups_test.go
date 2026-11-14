package api

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testBackup = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0770); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0660); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestBackupAndRestoreRoundTrip(t *testing.T) {
	root := useTempDataPath(t)
	server := filepath.Join(root, testService)

	writeTestFile(t, filepath.Join(server, "server.properties"), "motd=original\n")
	writeTestFile(t, filepath.Join(server, "world", "region", "r.0.0.mca"), "chunk-data")
	writeTestFile(t, filepath.Join(server, installedMarker), "installed")
	if err := os.MkdirAll(filepath.Join(server, "empty-dir"), 0770); err != nil {
		t.Fatal(err)
	}

	size, checksum, err := writeBackup(context.Background(), testService, testBackup)
	if err != nil {
		t.Fatal(err)
	}
	archive := backupPath(testService, testBackup)
	info, err := os.Stat(archive)
	if err != nil || info.Size() != size || len(checksum) != 64 {
		t.Fatalf("archive: err=%v size=%d reported=%d checksum=%q", err, info.Size(), size, checksum)
	}
	// The archive is kept with the files Atlas manages, outside the server
	// directory, so it is never part of a later backup of itself.
	if rel, _ := filepath.Rel(server, archive); !filepath.IsAbs(rel) && rel[:2] != ".." {
		t.Fatalf("the archive is inside the server directory: %s", archive)
	}
	if exists(archive + ".part") {
		t.Fatal("the temporary archive was left behind")
	}

	// The server changes after the backup.
	writeTestFile(t, filepath.Join(server, "server.properties"), "motd=changed\n")
	writeTestFile(t, filepath.Join(server, "new-file.txt"), "made later")
	if err := os.RemoveAll(filepath.Join(server, "world")); err != nil {
		t.Fatal(err)
	}

	// A plain restore brings back what was backed up and keeps newer files.
	if err := restoreBackup(context.Background(), testService, testBackup, false); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(server, "server.properties")); got != "motd=original\n" {
		t.Errorf("server.properties = %q", got)
	}
	if got := readTestFile(t, filepath.Join(server, "world", "region", "r.0.0.mca")); got != "chunk-data" {
		t.Errorf("world file = %q", got)
	}
	if !exists(filepath.Join(server, "empty-dir")) || !exists(filepath.Join(server, "new-file.txt")) {
		t.Error("an empty directory was lost or a newer file was removed by a plain restore")
	}

	// With truncate, only what is in the backup remains.
	writeTestFile(t, filepath.Join(server, "world", "junk.tmp"), "junk")
	if err := restoreBackup(context.Background(), testService, testBackup, true); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(server, "new-file.txt")) || exists(filepath.Join(server, "world", "junk.tmp")) {
		t.Error("files outside the backup survived a truncating restore")
	}
	if got := readTestFile(t, filepath.Join(server, "world", "region", "r.0.0.mca")); got != "chunk-data" {
		t.Errorf("world file after truncating restore = %q", got)
	}
	if !exists(filepath.Join(server, installedMarker)) {
		t.Error("the server no longer counts as installed after a restore")
	}
}

func TestRestoreKeepsAnInstalledServerInstalled(t *testing.T) {
	root := useTempDataPath(t)
	server := filepath.Join(root, testService)

	// A backup taken from a directory without the marker...
	writeTestFile(t, filepath.Join(server, "data.txt"), "data")
	if _, _, err := writeBackup(context.Background(), testService, testBackup); err != nil {
		t.Fatal(err)
	}
	// ...restored over a server that has since been marked installed.
	writeTestFile(t, filepath.Join(server, installedMarker), "installed")
	writeTestFile(t, filepath.Join(server, installFailedMarker), "old failure")

	if err := restoreBackup(context.Background(), testService, testBackup, true); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(server, installedMarker)) {
		t.Error("the installed marker was lost")
	}
	if exists(filepath.Join(server, installFailedMarker)) {
		t.Error("a stale failure marker survived the restore")
	}
}

func TestRestoreNeverWritesOutsideTheServerDirectory(t *testing.T) {
	root := useTempDataPath(t)
	server := filepath.Join(root, testService)
	writeTestFile(t, filepath.Join(server, "keep.txt"), "keep")
	outside := filepath.Join(root, "outside.txt")

	// An archive no backup would ever produce: entries that try to climb out.
	archive := backupPath(testService, testBackup)
	if err := os.MkdirAll(filepath.Dir(archive), 0700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	compressor := gzip.NewWriter(file)
	writer := tar.NewWriter(compressor)
	add := func(header *tar.Header, content string) {
		header.Size = int64(len(content))
		header.Mode = 0644
		header.ModTime = time.Now()
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	add(&tar.Header{Name: "../outside.txt", Typeflag: tar.TypeReg}, "escaped")
	add(&tar.Header{Name: "sub/../../outside.txt", Typeflag: tar.TypeReg}, "escaped")
	add(&tar.Header{Name: "/absolute.txt", Typeflag: tar.TypeReg}, "made relative")
	add(&tar.Header{Name: "normal.txt", Typeflag: tar.TypeReg}, "fine")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressor.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if err := restoreBackup(context.Background(), testService, testBackup, false); err != nil {
		t.Fatal(err)
	}
	if exists(outside) {
		t.Fatal("a backup entry was written outside the server directory")
	}
	if got := readTestFile(t, filepath.Join(server, "normal.txt")); got != "fine" {
		t.Errorf("normal.txt = %q", got)
	}
	// A leading slash is dropped, which keeps the file inside the server.
	if got := readTestFile(t, filepath.Join(server, "absolute.txt")); got != "made relative" {
		t.Errorf("absolute.txt = %q", got)
	}
	if !exists(filepath.Join(server, "keep.txt")) {
		t.Error("an existing file was removed by a plain restore")
	}
}

func TestRestoreRefusesToFollowALinkOutOfTheServerDirectory(t *testing.T) {
	root := useTempDataPath(t)
	server := filepath.Join(root, testService)
	outsideDir := filepath.Join(root, "outside")
	if err := os.MkdirAll(outsideDir, 0770); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(server, 0770); err != nil {
		t.Fatal(err)
	}
	// The server directory already holds a link that leads out of it.
	if err := os.Symlink(outsideDir, filepath.Join(server, "link")); err != nil {
		t.Skipf("symbolic links are not available here: %v", err)
	}

	archive := backupPath(testService, testBackup)
	if err := os.MkdirAll(filepath.Dir(archive), 0700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	compressor := gzip.NewWriter(file)
	writer := tar.NewWriter(compressor)
	content := "escaped"
	if err := writer.WriteHeader(&tar.Header{Name: "link/planted.txt", Typeflag: tar.TypeReg, Size: int64(len(content)), Mode: 0644}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	_ = compressor.Close()
	_ = file.Close()

	// The restore fails rather than writing through the link.
	if err := restoreBackup(context.Background(), testService, testBackup, false); err == nil {
		t.Error("a restore through an escaping link reported success")
	}
	if exists(filepath.Join(outsideDir, "planted.txt")) {
		t.Fatal("a backup entry was written through a link that leaves the server directory")
	}
}

func TestBackupRegistry(t *testing.T) {
	registry := &backupRegistry{jobs: make(map[string]*backupJob)}

	_, first, ok := registry.begin("backup-1", "server-a", false)
	if !ok {
		t.Fatal("the first backup was refused")
	}
	if _, _, ok := registry.begin("backup-1", "server-a", false); ok {
		t.Fatal("the same backup started twice")
	}
	// Nothing else may touch a server while a restore replaces its files,
	// and a restore may not start while a backup is reading them.
	if _, _, ok := registry.begin("backup-2", "server-a", true); ok {
		t.Fatal("a restore started while a backup of the same server was running")
	}
	_, other, ok := registry.begin("backup-3", "server-b", true)
	if !ok {
		t.Fatal("a restore on another server was refused")
	}
	if _, _, ok := registry.begin("backup-4", "server-b", false); ok {
		t.Fatal("a backup started while its server was being restored")
	}
	if !registry.restoring("server-b") || registry.restoring("server-a") {
		t.Fatal("restoring() reports the wrong servers")
	}
	if got := registry.list(); len(got) != 2 || got[0] != "backup-1" || got[1] != "backup-3" {
		t.Fatalf("list = %v", got)
	}

	registry.finish("backup-1", first)
	registry.finish("backup-3", other)
	if len(registry.list()) != 0 || registry.restoring("server-b") {
		t.Fatal("finished jobs are still listed")
	}
	if _, again, ok := registry.begin("backup-1", "server-a", false); !ok {
		t.Fatal("a finished backup's slot was not released")
	} else {
		registry.finish("backup-1", again)
	}
}

func TestCancellingABackupStopsItsJob(t *testing.T) {
	registry := &backupRegistry{jobs: make(map[string]*backupJob)}
	ctx, job, _ := registry.begin("backup-1", "server-a", false)

	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		registry.finish("backup-1", job)
		close(done)
	}()

	registry.cancel("backup-1")
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the job did not stop")
	}
	if cause := context.Cause(ctx); cause != errBackupCancelled {
		t.Fatalf("cause = %v", cause)
	}
	// Cancelling something that is not running is harmless.
	registry.cancel("backup-1")
	registry.cancelService("server-a")
}
