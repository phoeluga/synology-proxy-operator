package synology

import "time"

// ProxyRecord represents a reverse proxy record
type ProxyRecord struct {
	UUID             string
	FrontendHostname string
	FrontendPort     int
	FrontendProtocol string // "http" or "https"
	BackendHostname  string
	BackendPort      int
	BackendProtocol  string // "http" or "https"
	CertificateID    string
	ACLProfileName   string
	Description      string
	Enabled          bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Validate validates the proxy record
func (p *ProxyRecord) Validate() error {
	if p.FrontendHostname == "" {
		return &ValidationError{Field: "FrontendHostname", Message: "frontend hostname is required"}
	}
	if p.FrontendPort <= 0 || p.FrontendPort > 65535 {
		return &ValidationError{Field: "FrontendPort", Message: "frontend port must be between 1 and 65535"}
	}
	if p.FrontendProtocol != "http" && p.FrontendProtocol != "https" {
		return &ValidationError{Field: "FrontendProtocol", Message: "frontend protocol must be http or https"}
	}
	if p.BackendHostname == "" {
		return &ValidationError{Field: "BackendHostname", Message: "backend hostname is required"}
	}
	if p.BackendPort <= 0 || p.BackendPort > 65535 {
		return &ValidationError{Field: "BackendPort", Message: "backend port must be between 1 and 65535"}
	}
	if p.BackendProtocol != "http" && p.BackendProtocol != "https" {
		return &ValidationError{Field: "BackendProtocol", Message: "backend protocol must be http or https"}
	}
	return nil
}

// Certificate represents an SSL/TLS certificate
type Certificate struct {
	ID              string
	Name            string
	Desc            string // Description/Common Name for display
	CommonName      string
	SubjectAltNames []string
	Issuer          string
	Subject         string
	ValidFrom       time.Time
	ValidUntil      time.Time
	IsWildcard      bool
	IsDefault       bool
}

// MatchesHost checks if the certificate matches the given hostname
func (c *Certificate) MatchesHost(hostname string) bool {
	// Exact match with Desc field
	if c.Desc == hostname {
		return true
	}
	
	// Exact match with CommonName
	if c.CommonName == hostname {
		return true
	}
	
	// Wildcard match with Desc
	if len(c.Desc) > 2 && c.Desc[0] == '*' && c.Desc[1] == '.' {
		domain := c.Desc[2:]
		if len(hostname) > len(domain) && hostname[len(hostname)-len(domain):] == domain {
			// Check it's a single-level subdomain
			prefix := hostname[:len(hostname)-len(domain)-1]
			if len(prefix) > 0 && !contains(prefix, '.') {
				return true
			}
		}
	}
	
	// Wildcard match with CommonName
	if len(c.CommonName) > 2 && c.CommonName[0] == '*' && c.CommonName[1] == '.' {
		domain := c.CommonName[2:]
		if len(hostname) > len(domain) && hostname[len(hostname)-len(domain):] == domain {
			prefix := hostname[:len(hostname)-len(domain)-1]
			if len(prefix) > 0 && !contains(prefix, '.') {
				return true
			}
		}
	}
	
	// Check SubjectAltNames
	for _, san := range c.SubjectAltNames {
		if san == hostname {
			return true
		}
		if len(san) > 2 && san[0] == '*' && san[1] == '.' {
			domain := san[2:]
			if len(hostname) > len(domain) && hostname[len(hostname)-len(domain):] == domain {
				prefix := hostname[:len(hostname)-len(domain)-1]
				if len(prefix) > 0 && !contains(prefix, '.') {
					return true
				}
			}
		}
	}
	
	return false
}

func contains(s string, ch byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == ch {
			return true
		}
	}
	return false
}

// ACLProfile represents an access control list profile
type ACLProfile struct {
	ID          string
	Name        string
	Description string
	Rules       []ACLRule
	IsDefault   bool
}

// ACLRule represents a single ACL rule
type ACLRule struct {
	Type  string // "allow" or "deny"
	Value string // IP address or CIDR
}

// Session represents an authenticated session
type Session struct {
	SID       string
	SynoToken string
	CreatedAt time.Time
	IsValid   bool
}

// API Request/Response structures

type authRequest struct {
	Account string `json:"account"`
	Passwd  string `json:"passwd"`
	Session string `json:"session"`
	Format  string `json:"format"`
}

type authResponse struct {
	Success bool `json:"success"`
	Data    struct {
		SID       string `json:"sid"`
		SynoToken string `json:"synotoken"`
	} `json:"data"`
	Error struct {
		Code int `json:"code"`
	} `json:"error"`
}

type proxyListRequest struct {
	API     string `json:"api"`
	Version int    `json:"version"`
	Method  string `json:"method"`
}

type proxyListResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Records []proxyRecordAPI `json:"records"`
	} `json:"data"`
	Error struct {
		Code int `json:"code"`
	} `json:"error"`
}

type proxyRecordAPI struct {
	UUID             string `json:"uuid"`
	FrontendHostname string `json:"frontend_hostname"`
	FrontendPort     int    `json:"frontend_port"`
	FrontendProtocol string `json:"frontend_protocol"`
	BackendHostname  string `json:"backend_hostname"`
	BackendPort      int    `json:"backend_port"`
	BackendProtocol  string `json:"backend_protocol"`
	CertificateID    string `json:"certificate_id"`
	ACLProfileName   string `json:"acl_profile_name"`
	Description      string `json:"description"`
	Enabled          bool   `json:"enabled"`
}

type certificateListResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Certificates []certificateAPI `json:"certificates"`
	} `json:"data"`
	Error struct {
		Code int `json:"code"`
	} `json:"error"`
}

type certificateAPI struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	CommonName      string   `json:"common_name"`
	SubjectAltNames []string `json:"subject_alt_names"`
	Issuer          string   `json:"issuer"`
	ValidFrom       int64    `json:"valid_from"`
	ValidUntil      int64    `json:"valid_until"`
	IsDefault       bool     `json:"is_default"`
}

type aclListResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Profiles []aclProfileAPI `json:"profiles"`
	} `json:"data"`
	Error struct {
		Code int `json:"code"`
	} `json:"error"`
}

type aclProfileAPI struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsDefault   bool   `json:"is_default"`
}

// Helper functions for type conversion

func (p *ProxyRecord) toAPI() *proxyRecordAPI {
	return &proxyRecordAPI{
		UUID:             p.UUID,
		FrontendHostname: p.FrontendHostname,
		FrontendPort:     p.FrontendPort,
		FrontendProtocol: p.FrontendProtocol,
		BackendHostname:  p.BackendHostname,
		BackendPort:      p.BackendPort,
		BackendProtocol:  p.BackendProtocol,
		CertificateID:    p.CertificateID,
		ACLProfileName:   p.ACLProfileName,
		Description:      p.Description,
		Enabled:          p.Enabled,
	}
}

func (api *proxyRecordAPI) toDomain() *ProxyRecord {
	return &ProxyRecord{
		UUID:             api.UUID,
		FrontendHostname: api.FrontendHostname,
		FrontendPort:     api.FrontendPort,
		FrontendProtocol: api.FrontendProtocol,
		BackendHostname:  api.BackendHostname,
		BackendPort:      api.BackendPort,
		BackendProtocol:  api.BackendProtocol,
		CertificateID:    api.CertificateID,
		ACLProfileName:   api.ACLProfileName,
		Description:      api.Description,
		Enabled:          api.Enabled,
	}
}

func (api *certificateAPI) toDomain() *Certificate {
	cert := &Certificate{
		ID:              api.ID,
		Name:            api.Name,
		Desc:            api.CommonName, // Use CommonName as Desc
		CommonName:      api.CommonName,
		Subject:         api.CommonName,
		SubjectAltNames: api.SubjectAltNames,
		Issuer:          api.Issuer,
		ValidFrom:       time.Unix(api.ValidFrom, 0),
		ValidUntil:      time.Unix(api.ValidUntil, 0),
		IsDefault:       api.IsDefault,
	}
	
	// Check if wildcard certificate
	if len(cert.CommonName) > 0 && cert.CommonName[0] == '*' {
		cert.IsWildcard = true
	}
	for _, san := range cert.SubjectAltNames {
		if len(san) > 0 && san[0] == '*' {
			cert.IsWildcard = true
			break
		}
	}
	
	return cert
}

func (api *aclProfileAPI) toDomain() *ACLProfile {
	return &ACLProfile{
		ID:          api.Name, // Use Name as ID
		Name:        api.Name,
		Description: api.Description,
		IsDefault:   api.IsDefault,
		Rules:       []ACLRule{}, // Rules would need to be fetched separately
	}
}
