package handlers

import (
	"fmt"
	"strings"
	"testing"
)

func TestNodeErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		fallback string
		want     string
	}{
		{
			name:     "preserves daemon error",
			body:     `{"error":"Failed to start server container: port is already allocated"}`,
			fallback: "generic error",
			want:     "Failed to start server container: port is already allocated",
		},
		{
			name:     "trims daemon error",
			body:     `{"error":"  permission denied  "}`,
			fallback: "generic error",
			want:     "permission denied",
		},
		{
			name:     "uses fallback for malformed response",
			body:     `<html>bad gateway</html>`,
			fallback: "generic error",
			want:     "generic error",
		},
		{
			name:     "uses fallback for empty error",
			body:     `{"error":""}`,
			fallback: "generic error",
			want:     "generic error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := nodeErrorMessage(strings.NewReader(test.body), test.fallback); got != test.want {
				t.Fatalf("nodeErrorMessage() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestValidateServicePort(t *testing.T) {
	tests := []struct {
		port    int
		wantErr bool
	}{
		{port: 0, wantErr: true},
		{port: 1, wantErr: false},
		{port: 27015, wantErr: false},
		{port: 65535, wantErr: false},
		{port: 65536, wantErr: true},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("port_%d", test.port), func(t *testing.T) {
			err := validateServicePort(test.port)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateServicePort(%d) error = %v, wantErr %t", test.port, err, test.wantErr)
			}
		})
	}
}
