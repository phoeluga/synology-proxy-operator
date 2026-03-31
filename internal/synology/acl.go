package synology

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

const aclEndpoint = "/webapi/entry.cgi/SYNO.Core.AppPortal.AccessControl"

// GetACLProfileID returns the UUID of the named access control profile,
// or an empty string if no profile with that name is found.
func (c *Client) GetACLProfileID(ctx context.Context, profileName string) (string, error) {
	if profileName == "" {
		return "", nil
	}

	data, err := c.post(ctx, aclEndpoint, url.Values{
		"api":     {"SYNO.Core.AppPortal.AccessControl"},
		"method":  {"list"},
		"version": {"1"},
	})
	if err != nil {
		return "", fmt.Errorf("listing ACL profiles: %w", err)
	}

	var result struct {
		Entries []ACLProfile `json:"entries"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parsing ACL profile list: %w", err)
	}

	for _, p := range result.Entries {
		if p.Name == profileName {
			return p.UUID, nil
		}
	}
	return "", fmt.Errorf("ACL profile %q not found in DSM", profileName)
}
