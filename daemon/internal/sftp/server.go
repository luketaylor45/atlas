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

	"github.com/luketaylor45/atlas/daemon/internal/config"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTPServer struct {
	Port    string
	DataDir string
}

type AuthValidator func(username, password string) (bool, string) // Returns (valid, serviceUUID)

var authCallback AuthValidator

func NewServer(port, dataDir string) *SFTPServer {
	return &SFTPServer{
		Port:    port,
		DataDir: dataDir,
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
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			username := c.User()
			password := string(pass)

			// Try user-based authentication first (via Core API)
			if authCallback != nil {
				if valid, uuid := authCallback(username, password); valid {
					log.Printf("[SFTP] ✓ User Authenticated: %s (Service: %s)", username, uuid)

					// Verify service directory exists
					serviceDir := filepath.Join(s.DataDir, uuid)
					if _, err := os.Stat(serviceDir); os.IsNotExist(err) {
						log.Printf("[SFTP] Service directory not found: %s", uuid)
						return nil, fmt.Errorf("service not found")
					}

					return &ssh.Permissions{
						Extensions: map[string]string{
							"uuid":     uuid,
							"username": username,
						},
					}, nil
				}
			}

			// Fallback: Legacy format service_<UUID> with node token
			if strings.HasPrefix(username, "service_") && password == config.NodeConfig.NodeToken {
				uuid := strings.TrimPrefix(username, "service_")
				serviceDir := filepath.Join(s.DataDir, uuid)
				if _, err := os.Stat(serviceDir); os.IsNotExist(err) {
					log.Printf("[SFTP] Service directory not found: %s", uuid)
					return nil, fmt.Errorf("service not found")
				}

				log.Printf("[SFTP] ✓ Legacy Auth: %s", username)
				return &ssh.Permissions{
					Extensions: map[string]string{
						"uuid":     uuid,
						"username": "legacy",
					},
				}, nil
			}

			log.Printf("[SFTP] Authentication failed for: %s", username)
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

		// Serve SFTP
		if err := server.Serve(); err == io.EOF {
			server.Close()
			log.Printf("[SFTP] Connection closed: User=%s, Service=%s", username, uuid)
		} else if err != nil {
			log.Printf("[SFTP] Server error: %v", err)
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
