package synology

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProxyRecord_Validate(t *testing.T) {
	tests := []struct {
		name    string
		record  *ProxyRecord
		wantErr bool
	}{
		{
			name: "valid record",
			record: &ProxyRecord{
				FrontendHostname: "example.com",
				FrontendPort:     443,
				FrontendProtocol: "https",
				BackendHostname:  "backend.local",
				BackendPort:      8080,
				BackendProtocol:  "http",
			},
			wantErr: false,
		},
		{
			name: "missing frontend hostname",
			record: &ProxyRecord{
				FrontendPort:     443,
				FrontendProtocol: "https",
				BackendHostname:  "backend.local",
				BackendPort:      8080,
			},
			wantErr: true,
		},
		{
			name: "invalid frontend port",
			record: &ProxyRecord{
				FrontendHostname: "example.com",
				FrontendPort:     0,
				FrontendProtocol: "https",
				BackendHostname:  "backend.local",
				BackendPort:      8080,
			},
			wantErr: true,
		},
		{
			name: "invalid protocol",
			record: &ProxyRecord{
				FrontendHostname: "example.com",
				FrontendPort:     443,
				FrontendProtocol: "ftp",
				BackendHostname:  "backend.local",
				BackendPort:      8080,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.record.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCertificate_MatchesHost(t *testing.T) {
	tests := []struct {
		name     string
		cert     *Certificate
		hostname string
		want     bool
	}{
		{
			name:     "exact match",
			cert:     &Certificate{Desc: "example.com"},
			hostname: "example.com",
			want:     true,
		},
		{
			name:     "wildcard match",
			cert:     &Certificate{Desc: "*.example.com"},
			hostname: "sub.example.com",
			want:     true,
		},
		{
			name:     "wildcard no match - different domain",
			cert:     &Certificate{Desc: "*.example.com"},
			hostname: "other.com",
			want:     false,
		},
		{
			name:     "wildcard no match - too many levels",
			cert:     &Certificate{Desc: "*.example.com"},
			hostname: "a.b.example.com",
			want:     false,
		},
		{
			name:     "no match",
			cert:     &Certificate{Desc: "example.com"},
			hostname: "other.com",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cert.MatchesHost(tt.hostname)
			assert.Equal(t, tt.want, got)
		})
	}
}
