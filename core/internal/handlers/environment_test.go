package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/luketaylor45/atlas/core/internal/models"
)

func testEgg() *models.Egg {
	return &models.Egg{Variables: []models.EggVariable{
		{Name: "Map", EnvironmentVariable: "MAP", DefaultValue: "gm_flatgrass", UserEditable: true},
		{Name: "Tickrate", EnvironmentVariable: "TICKRATE", DefaultValue: "22", UserEditable: false},
	}}
}

func decodeEnv(t *testing.T, raw string) map[string]string {
	t.Helper()
	values := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		t.Fatal(err)
	}
	return values
}

func TestValidateEnvironmentOverrides(t *testing.T) {
	tests := []struct {
		name      string
		submitted string
		current   string
		admin     bool
		wantErr   string
		want      map[string]string
	}{
		{
			name:      "editable variable changes and defaults are filled",
			submitted: `{"MAP":"rp_city"}`,
			want:      map[string]string{"MAP": "rp_city", "TICKRATE": "22"},
		},
		{
			name:      "locked variable is refused for users",
			submitted: `{"TICKRATE":"66"}`,
			wantErr:   "Tickrate cannot be changed",
		},
		{
			name:      "locked variable is allowed for admins",
			submitted: `{"TICKRATE":"66"}`,
			admin:     true,
			want:      map[string]string{"MAP": "gm_flatgrass", "TICKRATE": "66"},
		},
		{
			name:      "resubmitting a locked value unchanged is fine",
			submitted: `{"MAP":"rp_city","TICKRATE":"33"}`,
			current:   `{"TICKRATE":"33"}`,
			want:      map[string]string{"MAP": "rp_city", "TICKRATE": "33"},
		},
		{
			name:      "new unknown variable is refused",
			submitted: `{"LD_PRELOAD":"/tmp/x.so"}`,
			admin:     true,
			wantErr:   "unknown variable",
		},
		{
			name:      "stored legacy variable survives when resubmitted unchanged",
			submitted: `{"OLD":"1","MAP":"rp_city"}`,
			current:   `{"OLD":"1"}`,
			want:      map[string]string{"OLD": "1", "MAP": "rp_city", "TICKRATE": "22"},
		},
		{
			name:      "multi-line values are refused",
			submitted: `{"MAP":"a\nb"}`,
			wantErr:   "single line",
		},
		{
			name:      "invalid JSON is refused instead of silently dropped",
			submitted: `{not json`,
			wantErr:   "JSON object",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := validateEnvironmentOverrides(test.submitted, testEgg(), test.current, test.admin)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			values := decodeEnv(t, got)
			if len(values) != len(test.want) {
				t.Fatalf("got %v, want %v", values, test.want)
			}
			for key, want := range test.want {
				if values[key] != want {
					t.Fatalf("%s = %q, want %q (all: %v)", key, values[key], want, values)
				}
			}
		})
	}
}

func TestNodeStatusNeverLooksLikeAnExpiredSession(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusServiceUnavailable} {
		if got := nodeStatus(code); got != http.StatusBadGateway {
			t.Fatalf("nodeStatus(%d) = %d, want 502", code, got)
		}
	}
	if got := nodeStatus(http.StatusNotFound); got != http.StatusNotFound {
		t.Fatalf("nodeStatus(404) = %d", got)
	}
}

func TestNodeHostPrefersInternalAddress(t *testing.T) {
	node := &models.Node{Address: "panel.example.com", Port: "8081"}
	if got := nodeHost(node); got != "panel.example.com:8081" {
		t.Fatalf("nodeHost = %q", got)
	}
	node.InternalAddress = "daemon"
	if got := nodeHost(node); got != "daemon:8081" {
		t.Fatalf("nodeHost with internal address = %q", got)
	}
}

func TestSFTPServicePrefixPattern(t *testing.T) {
	valid := []string{"14778ff7", "14778ff7-a24f-43c5-b6d3-ec2febce7177"}
	invalid := []string{"", "%", "1477", "14778ff_", "14778ff7%", "service_x"}
	for _, value := range valid {
		if !sftpServicePrefixPattern.MatchString(value) {
			t.Fatalf("%q should be accepted", value)
		}
	}
	for _, value := range invalid {
		if sftpServicePrefixPattern.MatchString(value) {
			t.Fatalf("%q should be rejected", value)
		}
	}
}
