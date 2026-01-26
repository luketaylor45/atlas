package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/luketaylor45/atlas/daemon/internal/config"
)

type SFTPAuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type SFTPAuthResponse struct {
	Valid       bool   `json:"valid"`
	ServiceUUID string `json:"service_uuid"`
}

// ValidateSFTPCredentials calls Core API to validate SFTP login
func ValidateSFTPCredentials(username, password string) (bool, string) {
	reqBody := SFTPAuthRequest{
		Username: username,
		Password: password,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("[SFTP-Auth] Failed to marshal request: %v", err)
		return false, ""
	}

	// Call Core API
	url := fmt.Sprintf("%s/api/v1/internal/sftp/validate", config.NodeConfig.CoreURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[SFTP-Auth] Failed to create request: %v", err)
		return false, ""
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Token", config.NodeConfig.NodeToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[SFTP-Auth] Failed to contact Core: %v", err)
		return false, ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, ""
	}

	var authResp SFTPAuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return false, ""
	}

	return authResp.Valid, authResp.ServiceUUID
}
