package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/daemon/internal/config"
)

func writeTestZip(t *testing.T, archivePath string, entries map[string]string) {
	t.Helper()

	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create test archive: %v", err)
	}
	writer := zip.NewWriter(archiveFile)
	for name, contents := range entries {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			t.Fatalf("create archive entry: %v", err)
		}
		if _, err := entry.Write([]byte(contents)); err != nil {
			t.Fatalf("write archive entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close archive writer: %v", err)
	}
	if err := archiveFile.Close(); err != nil {
		t.Fatalf("close archive file: %v", err)
	}
}

func testArchiveFiles() archiveFileOperations {
	return archiveFileOperations{
		ensureDirectory: func(path string) error {
			return os.MkdirAll(path, 0o770)
		},
		normalizePath: func(string) error { return nil },
	}
}

func TestExtractZipArchive(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "bundle.zip")
	writeTestZip(t, archivePath, map[string]string{
		"config/server.cfg": "hostname Atlas\n",
		"README.txt":        "ready",
	})

	extracted, err := extractZipArchive(archivePath, root, testArchiveFiles())
	if err != nil {
		t.Fatalf("extract archive: %v", err)
	}
	if extracted != 2 {
		t.Fatalf("expected 2 extracted entries, got %d", extracted)
	}
	contents, err := os.ReadFile(filepath.Join(root, "config", "server.cfg"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(contents) != "hostname Atlas\n" {
		t.Fatalf("unexpected extracted contents: %q", contents)
	}
}

func TestExtractZipArchiveRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "malicious.zip")
	writeTestZip(t, archivePath, map[string]string{"../escaped.txt": "nope"})

	if _, err := extractZipArchive(archivePath, root, testArchiveFiles()); !errors.Is(err, errInvalidZipArchive) {
		t.Fatalf("expected invalid archive error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escaped.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("archive escaped extraction root: %v", err)
	}
}

func TestExtractZipArchiveRejectsSymlinkEntry(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "symlink.zip")
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create test archive: %v", err)
	}
	writer := zip.NewWriter(archiveFile)
	header := &zip.FileHeader{Name: "linked-file"}
	header.SetMode(os.ModeSymlink | 0o777)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatalf("create symlink entry: %v", err)
	}
	if _, err := entry.Write([]byte("../outside")); err != nil {
		t.Fatalf("write symlink entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close archive writer: %v", err)
	}
	if err := archiveFile.Close(); err != nil {
		t.Fatalf("close archive file: %v", err)
	}

	if _, err := extractZipArchive(archivePath, root, testArchiveFiles()); !errors.Is(err, errInvalidZipArchive) {
		t.Fatalf("expected invalid archive error, got %v", err)
	}
}

func TestExtractZipArchiveRejectsExistingSymlinkPath(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}

	archivePath := filepath.Join(root, "symlink-path.zip")
	writeTestZip(t, archivePath, map[string]string{"linked/escaped.txt": "nope"})
	if _, err := extractZipArchive(archivePath, root, testArchiveFiles()); !errors.Is(err, errInvalidZipArchive) {
		t.Fatalf("expected invalid archive error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "escaped.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("archive wrote through existing symlink: %v", err)
	}
}

func TestExtractZipArchiveDoesNotOverwriteWithCorruptEntry(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "corrupt.zip")
	writeTestZip(t, archivePath, map[string]string{"server.cfg": "replace"})

	archiveBytes, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read test archive: %v", err)
	}
	corrupted := bytes.Replace(archiveBytes, []byte("replace"), []byte("corrupt"), 1)
	if bytes.Equal(corrupted, archiveBytes) {
		t.Fatal("failed to locate archive payload to corrupt")
	}
	if err := os.WriteFile(archivePath, corrupted, 0o660); err != nil {
		t.Fatalf("write corrupt archive: %v", err)
	}

	targetPath := filepath.Join(root, "server.cfg")
	if err := os.WriteFile(targetPath, []byte("keep"), 0o660); err != nil {
		t.Fatalf("write existing target: %v", err)
	}
	if _, err := extractZipArchive(archivePath, root, testArchiveFiles()); err == nil {
		t.Fatal("expected corrupt archive extraction to fail")
	}
	contents, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read existing target: %v", err)
	}
	if string(contents) != "keep" {
		t.Fatalf("corrupt archive replaced existing file with %q", contents)
	}
}

func TestSecurePathRejectsTraversal(t *testing.T) {
	previousConfig := config.NodeConfig
	t.Cleanup(func() { config.NodeConfig = previousConfig })
	config.NodeConfig.DataPath = t.TempDir()

	if _, err := securePath("../outside", "file.txt"); err == nil {
		t.Fatal("expected service UUID traversal to be rejected")
	}
	if _, err := securePath("server", "../outside.txt"); err == nil {
		t.Fatal("expected file traversal to be rejected")
	}

	want := filepath.Join(config.NodeConfig.DataPath, "server", "config", "server.cfg")
	got, err := securePath("server", "config/server.cfg")
	if err != nil {
		t.Fatalf("expected valid path, got %v", err)
	}
	if got != want {
		t.Fatalf("securePath() = %q, want %q", got, want)
	}
}

func TestExtractArchiveHandlerRejectsNonZip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousConfig := config.NodeConfig
	t.Cleanup(func() { config.NodeConfig = previousConfig })
	config.NodeConfig.NodeToken = "test-node-token"

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/servers/server/files/extract", bytes.NewBufferString(`{"path":"server.tar.gz"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Header.Set("X-Node-Token", config.NodeConfig.NodeToken)
	context.Params = gin.Params{{Key: "uuid", Value: "server"}}

	ExtractArchive(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
}

func TestExtractArchiveHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousConfig := config.NodeConfig
	previousArchiveFiles := managedArchiveFiles
	t.Cleanup(func() {
		config.NodeConfig = previousConfig
		managedArchiveFiles = previousArchiveFiles
	})

	const (
		nodeToken = "test-node-token"
		serverID  = "server"
	)
	config.NodeConfig.DataPath = t.TempDir()
	config.NodeConfig.NodeToken = nodeToken
	managedArchiveFiles = testArchiveFiles()
	serverRoot := filepath.Join(config.NodeConfig.DataPath, serverID)
	if err := os.MkdirAll(serverRoot, 0o770); err != nil {
		t.Fatalf("create server root: %v", err)
	}
	writeTestZip(t, filepath.Join(serverRoot, "bundle.zip"), map[string]string{"config.cfg": "enabled=true"})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/servers/"+serverID+"/files/extract", bytes.NewBufferString(`{"path":"bundle.zip"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Header.Set("X-Node-Token", nodeToken)
	context.Params = gin.Params{{Key: "uuid", Value: serverID}}

	ExtractArchive(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response struct {
		Extracted int `json:"extracted"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Extracted != 1 {
		t.Fatalf("expected 1 extracted entry, got %d", response.Extracted)
	}
	contents, err := os.ReadFile(filepath.Join(serverRoot, "config.cfg"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(contents) != "enabled=true" {
		t.Fatalf("unexpected extracted contents: %q", contents)
	}
}

func TestListFilesReturnsArrayForEmptyDirectory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	previousConfig := config.NodeConfig
	t.Cleanup(func() {
		config.NodeConfig = previousConfig
	})

	const (
		nodeToken = "test-node-token"
		serverID  = "empty-server"
	)
	config.NodeConfig.DataPath = t.TempDir()
	config.NodeConfig.NodeToken = nodeToken

	if err := os.Mkdir(filepath.Join(config.NodeConfig.DataPath, serverID), 0o755); err != nil {
		t.Fatalf("create empty server directory: %v", err)
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/servers/"+serverID+"/files/list", nil)
	context.Request.Header.Set("X-Node-Token", nodeToken)
	context.Params = gin.Params{{Key: "uuid", Value: serverID}}

	ListFiles(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if body := strings.TrimSpace(recorder.Body.String()); body != "[]" {
		t.Fatalf("expected an empty JSON array, got %s", body)
	}
}
