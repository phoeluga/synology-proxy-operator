package synology

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MockServer is a mock Synology API server for testing
type MockServer struct {
	server       *httptest.Server
	mu           sync.Mutex
	records      map[string]*ProxyRecord
	certificates []*Certificate
	aclProfiles  []*ACLProfile
	failureMode  string
	delay        time.Duration
	sessionID    string
}

// NewMockServer creates a new mock server
func NewMockServer() *MockServer {
	mock := &MockServer{
		records:      make(map[string]*ProxyRecord),
		certificates: defaultCertificates(),
		aclProfiles:  defaultACLProfiles(),
		failureMode:  "none",
		sessionID:    "mock-session-id",
	}

	mock.server = httptest.NewServer(http.HandlerFunc(mock.handler))
	return mock
}

// URL returns the mock server URL
func (m *MockServer) URL() string {
	return m.server.URL
}

// Close closes the mock server
func (m *MockServer) Close() {
	m.server.Close()
}

// SetFailureMode sets the failure mode
func (m *MockServer) SetFailureMode(mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failureMode = mode
}

// SetDelay sets the response delay
func (m *MockServer) SetDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delay = delay
}

// handler handles HTTP requests
func (m *MockServer) handler(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	mode := m.failureMode
	delay := m.delay
	m.mu.Unlock()

	// Simulate delay
	if delay > 0 {
		time.Sleep(delay)
	}

	// Simulate failures
	switch mode {
	case "timeout":
		time.Sleep(35 * time.Second)
		return
	case "500":
		w.WriteHeader(http.StatusInternalServerError)
		return
	case "auth":
		w.WriteHeader(http.StatusUnauthorized)
		return
	case "404":
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Route to appropriate handler
	switch r.URL.Path {
	case "/webapi/auth.cgi":
		m.handleAuth(w, r)
	case "/webapi/entry.cgi":
		m.handleEntry(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// handleAuth handles authentication requests
func (m *MockServer) handleAuth(w http.ResponseWriter, r *http.Request) {
	api := r.URL.Query().Get("api")
	method := r.URL.Query().Get("method")

	if api == "SYNO.API.Auth" && method == "login" {
		m.handleLogin(w, r)
	} else if api == "SYNO.API.Auth" && method == "logout" {
		m.handleLogout(w, r)
	} else {
		writeError(w, 400, "Unknown API or method")
	}
}

// handleLogin handles login requests
func (m *MockServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("account")
	password := r.URL.Query().Get("passwd")

	if username == "admin" && password == "password" {
		response := map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"sid":        m.sessionID,
				"synotoken":  "mock-syno-token",
				"did":        "mock-device-id",
				"is_portal_port": false,
			},
		}
		writeJSON(w, response)
	} else {
		writeError(w, 400, "Invalid credentials")
	}
}

// handleLogout handles logout requests
func (m *MockServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"success": true,
	}
	writeJSON(w, response)
}

// handleEntry handles API entry requests
func (m *MockServer) handleEntry(w http.ResponseWriter, r *http.Request) {
	api := r.URL.Query().Get("api")
	method := r.URL.Query().Get("method")

	switch api {
	case "SYNO.Core.Application.ReverseProxy":
		m.handleReverseProxy(w, r, method)
	case "SYNO.Core.Certificate":
		m.handleCertificate(w, r, method)
	case "SYNO.Core.ACL":
		m.handleACL(w, r, method)
	default:
		writeError(w, 400, "Unknown API")
	}
}

// handleReverseProxy handles reverse proxy API requests
func (m *MockServer) handleReverseProxy(w http.ResponseWriter, r *http.Request, method string) {
	switch method {
	case "list":
		m.handleProxyList(w, r)
	case "get":
		m.handleProxyGet(w, r)
	case "create":
		m.handleProxyCreate(w, r)
	case "update":
		m.handleProxyUpdate(w, r)
	case "delete":
		m.handleProxyDelete(w, r)
	default:
		writeError(w, 400, "Unknown method")
	}
}

// handleProxyList lists all proxy records
func (m *MockServer) handleProxyList(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	records := make([]*ProxyRecord, 0, len(m.records))
	for _, record := range m.records {
		records = append(records, record)
	}

	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"records": records,
		},
	}
	writeJSON(w, response)
}

// handleProxyGet gets a specific proxy record
func (m *MockServer) handleProxyGet(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")

	m.mu.Lock()
	record, exists := m.records[uuid]
	m.mu.Unlock()

	if !exists {
		writeError(w, 404, "Record not found")
		return
	}

	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"record": record,
		},
	}
	writeJSON(w, response)
}

// handleProxyCreate creates a new proxy record
func (m *MockServer) handleProxyCreate(w http.ResponseWriter, r *http.Request) {
	var record ProxyRecord
	if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
		writeError(w, 400, "Invalid request body")
		return
	}

	// Validate record
	if err := record.Validate(); err != nil {
		writeError(w, 400, err.Error())
		return
	}

	// Generate UUID
	record.UUID = uuid.New().String()

	m.mu.Lock()
	m.records[record.UUID] = &record
	m.mu.Unlock()

	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"record": record,
		},
	}
	writeJSON(w, response)
}

// handleProxyUpdate updates an existing proxy record
func (m *MockServer) handleProxyUpdate(w http.ResponseWriter, r *http.Request) {
	var record ProxyRecord
	if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
		writeError(w, 400, "Invalid request body")
		return
	}

	m.mu.Lock()
	_, exists := m.records[record.UUID]
	if !exists {
		m.mu.Unlock()
		writeError(w, 404, "Record not found")
		return
	}

	m.records[record.UUID] = &record
	m.mu.Unlock()

	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"record": record,
		},
	}
	writeJSON(w, response)
}

// handleProxyDelete deletes a proxy record
func (m *MockServer) handleProxyDelete(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")

	m.mu.Lock()
	_, exists := m.records[uuid]
	if !exists {
		m.mu.Unlock()
		writeError(w, 404, "Record not found")
		return
	}

	delete(m.records, uuid)
	m.mu.Unlock()

	response := map[string]interface{}{
		"success": true,
	}
	writeJSON(w, response)
}

// handleCertificate handles certificate API requests
func (m *MockServer) handleCertificate(w http.ResponseWriter, r *http.Request, method string) {
	switch method {
	case "list":
		m.handleCertificateList(w, r)
	case "set":
		m.handleCertificateSet(w, r)
	default:
		writeError(w, 400, "Unknown method")
	}
}

// handleCertificateList lists all certificates
func (m *MockServer) handleCertificateList(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"certificates": m.certificates,
		},
	}
	writeJSON(w, response)
}

// handleCertificateSet assigns a certificate
func (m *MockServer) handleCertificateSet(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"success": true,
	}
	writeJSON(w, response)
}

// handleACL handles ACL API requests
func (m *MockServer) handleACL(w http.ResponseWriter, r *http.Request, method string) {
	switch method {
	case "list":
		m.handleACLList(w, r)
	case "get":
		m.handleACLGet(w, r)
	default:
		writeError(w, 400, "Unknown method")
	}
}

// handleACLList lists all ACL profiles
func (m *MockServer) handleACLList(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"profiles": m.aclProfiles,
		},
	}
	writeJSON(w, response)
}

// handleACLGet gets a specific ACL profile
func (m *MockServer) handleACLGet(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, profile := range m.aclProfiles {
		if profile.ID == id {
			response := map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"profile": profile,
				},
			}
			writeJSON(w, response)
			return
		}
	}

	writeError(w, 404, "ACL profile not found")
}

// Helper functions

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, code int, message string) {
	response := map[string]interface{}{
		"success": false,
		"error": map[string]interface{}{
			"code": code,
		},
		"message": message,
	}
	writeJSON(w, response)
}

func defaultCertificates() []*Certificate {
	return []*Certificate{
		{
			ID:        "cert1",
			Name:      "example.com",
			Desc:      "example.com",
			IsDefault: false,
			Issuer:    "Let's Encrypt",
			Subject:   "example.com",
		},
		{
			ID:        "cert2",
			Name:      "*.example.com",
			Desc:      "*.example.com",
			IsDefault: false,
			Issuer:    "Let's Encrypt",
			Subject:   "*.example.com",
		},
		{
			ID:        "cert3",
			Name:      "default",
			Desc:      "default",
			IsDefault: true,
			Issuer:    "Synology",
			Subject:   "synology.local",
		},
	}
}

func defaultACLProfiles() []*ACLProfile {
	return []*ACLProfile{
		{
			ID:   "profile1",
			Name: "Allow All",
			Rules: []ACLRule{
				{Type: "allow", Value: "0.0.0.0/0"},
			},
		},
		{
			ID:   "profile2",
			Name: "Internal Only",
			Rules: []ACLRule{
				{Type: "allow", Value: "10.0.0.0/8"},
				{Type: "allow", Value: "172.16.0.0/12"},
				{Type: "allow", Value: "192.168.0.0/16"},
			},
		},
	}
}
