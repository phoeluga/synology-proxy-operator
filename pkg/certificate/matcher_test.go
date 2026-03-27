package certificate

import (
	"testing"

	"github.com/phoeluga/synology-proxy-operator/pkg/synology"
	"github.com/stretchr/testify/assert"
)

func TestMatcher_FindBestMatch(t *testing.T) {
	matcher := NewMatcher()

	certificates := []*synology.Certificate{
		{ID: "cert1", Desc: "example.com"},
		{ID: "cert2", Desc: "*.example.com"},
		{ID: "cert3", Desc: "*.sub.example.com"},
		{ID: "cert4", Desc: "other.com"},
	}

	tests := []struct {
		name     string
		hostname string
		want     string
		wantOK   bool
	}{
		{
			name:     "exact match",
			hostname: "example.com",
			want:     "cert1",
			wantOK:   true,
		},
		{
			name:     "wildcard match",
			hostname: "sub.example.com",
			want:     "cert2",
			wantOK:   true,
		},
		{
			name:     "nested wildcard match",
			hostname: "deep.sub.example.com",
			want:     "cert3",
			wantOK:   true,
		},
		{
			name:     "no match",
			hostname: "nomatch.com",
			want:     "",
			wantOK:   false,
		},
		{
			name:     "prefer exact over wildcard",
			hostname: "example.com",
			want:     "cert1", // Should prefer exact match over wildcard
			wantOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, ok := matcher.FindBestMatch(tt.hostname, certificates)
			if tt.wantOK {
				assert.True(t, ok)
				assert.Equal(t, tt.want, cert.ID)
			} else {
				assert.False(t, ok)
			}
		})
	}
}

func TestMatcher_FindBestMatch_EmptyCertificates(t *testing.T) {
	matcher := NewMatcher()

	cert, ok := matcher.FindBestMatch("example.com", []*synology.Certificate{})
	assert.False(t, ok)
	assert.Nil(t, cert)
}

func TestMatcher_FindBestMatch_EmptyHostname(t *testing.T) {
	matcher := NewMatcher()

	certificates := []*synology.Certificate{
		{ID: "cert1", Desc: "example.com"},
	}

	cert, ok := matcher.FindBestMatch("", certificates)
	assert.False(t, ok)
	assert.Nil(t, cert)
}

func TestMatcher_MatchesWildcard(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		hostname string
		want     bool
	}{
		{
			name:     "single level wildcard match",
			pattern:  "*.example.com",
			hostname: "sub.example.com",
			want:     true,
		},
		{
			name:     "wildcard no match - different domain",
			pattern:  "*.example.com",
			hostname: "other.com",
			want:     false,
		},
		{
			name:     "wildcard no match - too many levels",
			pattern:  "*.example.com",
			hostname: "a.b.example.com",
			want:     false,
		},
		{
			name:     "wildcard no match - base domain",
			pattern:  "*.example.com",
			hostname: "example.com",
			want:     false,
		},
		{
			name:     "exact match (no wildcard)",
			pattern:  "example.com",
			hostname: "example.com",
			want:     true,
		},
		{
			name:     "exact no match",
			pattern:  "example.com",
			hostname: "sub.example.com",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesWildcard(tt.pattern, tt.hostname)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatcher_PreferExactMatch(t *testing.T) {
	matcher := NewMatcher()

	// Both exact and wildcard match available
	certificates := []*synology.Certificate{
		{ID: "wildcard", Desc: "*.example.com"},
		{ID: "exact", Desc: "sub.example.com"},
	}

	// Should prefer exact match
	cert, ok := matcher.FindBestMatch("sub.example.com", certificates)
	assert.True(t, ok)
	assert.Equal(t, "exact", cert.ID)
}

func TestMatcher_MultipleWildcards(t *testing.T) {
	matcher := NewMatcher()

	// Multiple wildcard matches - should return first match
	certificates := []*synology.Certificate{
		{ID: "wildcard1", Desc: "*.example.com"},
		{ID: "wildcard2", Desc: "*.example.com"},
	}

	cert, ok := matcher.FindBestMatch("sub.example.com", certificates)
	assert.True(t, ok)
	assert.NotNil(t, cert)
	// Should return one of the wildcards (first match)
	assert.Contains(t, []string{"wildcard1", "wildcard2"}, cert.ID)
}
