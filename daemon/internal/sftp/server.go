package sftp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/luketaylor45/atlas/daemon/internal/serverfs"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	sftpMaxFailures   = 6
	sftpFailureWindow = 10 * time.Minute
	sftpBlockDuration = 30 * time.Minute
)

type SFTPServer struct {
	Port    string
	DataDir string
	limiter *failureLimiter
}

type AuthValidator func(username, password string) (bool, string) // Returns (valid, serviceUUID)

var authCallback AuthValidator

func NewServer(port, dataDir string) *SFTPServer {
	return &SFTPServer{
		Port:    port,
		DataDir: dataDir,
		limiter: newFailureLimiter(sftpMaxFailures, sftpFailureWindow, sftpBlockDuration),
	}
}

// SetAuthValidator sets the authentication callback function
func SetAuthValidator(validator AuthValidator) {
	authCallback = validator
}

// Start begins listening for SFTP connections
func (s *SFTPServer) Start() error {
	// Generate or load host key
	hostKey, err := s.getOrCreateHostKey()
	if err != nil {
		return fmt.Errorf("failed to load host key: %v", err)
	}

	// SSH server configuration
	sshConfig := &ssh.ServerConfig{
		MaxAuthTries: 3,
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			username := c.User()
			host := hostOf(c.RemoteAddr())

			if s.limiter.blocked(host) {
				return nil, fmt.Errorf("too many failed attempts")
			}

			// Atlas usernames are always "<service id>.<username>". Anything else
			// is a scanner and is rejected without a round trip to Core.
			if authCallback != nil && strings.Contains(username, ".") {
				if valid, uuid := authCallback(username, string(pass)); valid {
					uuid = strings.TrimSpace(uuid)
					if !serverfs.IsServiceDirectoryName(uuid) {
						return nil, fmt.Errorf("service not found")
					}

					serviceDir := filepath.Join(s.DataDir, uuid)
					if _, err := os.Stat(serviceDir); err != nil {
						log.Printf("[SFTP] Service directory check failed: uuid=%q path=%q err=%v", uuid, serviceDir, err)
						return nil, fmt.Errorf("service not found")
					}

					s.limiter.succeed(host)
					log.Printf("[SFTP] User authenticated: %s (Service: %s)", username, uuid)
					return &ssh.Permissions{
						Extensions: map[string]string{
							"uuid":     uuid,
							"username": username,
						},
					}, nil
				}
			}

			if s.limiter.fail(host) {
				log.Printf("[SFTP] Blocking %s for %s after repeated failed logins", host, sftpBlockDuration)
			}
			return nil, fmt.Errorf("invalid credentials")
		},
	}

	sshConfig.AddHostKey(hostKey)

	// Start listening
	listener, err := net.Listen("tcp", "0.0.0.0:"+s.Port)
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %v", s.Port, err)
	}

	log.Printf("[SFTP] Server listening on port %s", s.Port)
	log.Printf("[SFTP] Authentication: Username-based with per-user passwords")

	// Accept connections
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("[SFTP] Failed to accept connection: %v", err)
				continue
			}

			// Blocked addresses are dropped before the SSH handshake.
			if s.limiter.blocked(hostOf(conn.RemoteAddr())) {
				conn.Close()
				continue
			}

			go s.handleConnection(conn, sshConfig)
		}
	}()

	return nil
}

func (s *SFTPServer) handleConnection(netConn net.Conn, config *ssh.ServerConfig) {
	defer netConn.Close()

	// Perform SSH handshake
	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, config)
	if err != nil {
		return // Silent fail for handshake errors (port scanners)
	}
	defer sshConn.Close()

	// Get UUID and username from permissions
	uuid := sshConn.Permissions.Extensions["uuid"]
	username := sshConn.Permissions.Extensions["username"]
	serviceRoot := filepath.Join(s.DataDir, uuid)
	if err := serverfs.RepairServiceDirectory(serviceRoot); err != nil {
		log.Printf("[SFTP] Failed to prepare service permissions for %s: %v", uuid, err)
		return
	}
	defer func() {
		if err := serverfs.NormalizeTree(serviceRoot); err != nil {
			log.Printf("[SFTP] Failed to normalize uploaded files for %s: %v", uuid, err)
		}
	}()

	log.Printf("[SFTP] Connection established: User=%s, Service=%s, IP=%s", username, uuid, netConn.RemoteAddr().String())

	// Discard out-of-band requests
	go ssh.DiscardRequests(reqs)

	// Handle channels
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("[SFTP] Could not accept channel: %v", err)
			continue
		}

		// Handle subsystem requests
		go func(in <-chan *ssh.Request) {
			for req := range in {
				ok := false
				switch req.Type {
				case "subsystem":
					if string(req.Payload[4:]) == "sftp" {
						ok = true
					}
				}
				req.Reply(ok, nil)
			}
		}(requests)

		// Create SFTP server with chroot to service directory
		server, err := sftp.NewServer(
			channel,
			sftp.WithServerWorkingDirectory(serviceRoot),
		)
		if err != nil {
			log.Printf("[SFTP] Failed to create SFTP server: %v", err)
			channel.Close()
			continue
		}

		// Serve SFTP. The final tree repair catches ownership and chmod changes
		// made by clients using the package's root-owned filesystem backend.
		serveErr := server.Serve()
		_ = server.Close()
		if err := serverfs.NormalizeTree(serviceRoot); err != nil {
			log.Printf("[SFTP] Failed to normalize uploaded files for %s: %v", uuid, err)
		}
		if serveErr == io.EOF {
			log.Printf("[SFTP] Connection closed: User=%s, Service=%s", username, uuid)
		} else if serveErr != nil {
			log.Printf("[SFTP] Server error: %v", serveErr)
		}
	}
}

func (s *SFTPServer) getOrCreateHostKey() (ssh.Signer, error) {
	keyPath := filepath.Join(s.DataDir, ".ssh_host_key")

	// Try to load existing key
	if keyData, err := os.ReadFile(keyPath); err == nil {
		signer, err := ssh.ParsePrivateKey(keyData)
		if err == nil {
			log.Println("[SFTP] Loaded existing host key")
			return signer, nil
		}
	}

	// Generate new RSA key
	log.Println("[SFTP] Generating new RSA host key (2048-bit)...")

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %v", err)
	}

	// Encode to PEM format
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	privateKeyBytes := pem.EncodeToMemory(privateKeyPEM)

	// Parse to SSH signer
	signer, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated key: %v", err)
	}

	// Save for future use
	os.MkdirAll(filepath.Dir(keyPath), 0700)
	if err := os.WriteFile(keyPath, privateKeyBytes, 0600); err != nil {
		log.Printf("[SFTP] Warning: Could not save host key: %v", err)
	} else {
		log.Printf("[SFTP] Host key saved to: %s", keyPath)
	}

	return signer, nil
}
