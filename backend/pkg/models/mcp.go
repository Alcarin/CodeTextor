/*
  File: mcp.go
  Purpose: Shared MCP server configuration and telemetry models.
  Author: CodeTextor project
  Notes: These types are exported so Wails can generate frontend bindings.
*/

package models

// MCPServerProtocol enumerates available MCP transports.
type MCPServerProtocol string

const (
	// MCPProtocolHTTP serves MCP over the streamable HTTP transport.
	MCPProtocolHTTP MCPServerProtocol = "http"
	// MCPProtocolStdio serves MCP over stdio (not yet implemented).
	MCPProtocolStdio MCPServerProtocol = "stdio"
)

// MCPServerConfig stores runtime configuration for the MCP server.
type MCPServerConfig struct {
	Host           string            `json:"host"`
	Port           int               `json:"port"`
	Protocol       MCPServerProtocol `json:"protocol"`
	AutoStart      bool              `json:"autoStart"`
	MaxConnections int               `json:"maxConnections"`
}

// DefaultMCPServerConfig returns the initial configuration used on first run.
func DefaultMCPServerConfig() MCPServerConfig {
	return MCPServerConfig{
		Host:           "127.0.0.1",
		Port:           3030,
		Protocol:       MCPProtocolHTTP,
		AutoStart:      false,
		MaxConnections: 32,
	}
}

// MCPServerStatus describes runtime metrics for the MCP server.
type MCPServerStatus struct {
	IsRunning           bool    `json:"isRunning"`
	Uptime              int64   `json:"uptime"`
	ActiveConnections   int     `json:"activeConnections"`
	TotalRequests       int64   `json:"totalRequests"`
	AverageResponseTime float64 `json:"averageResponseTime"`
	LastError           string  `json:"lastError,omitempty"`
}

// MCPTool reports metadata for a registered tool along with usage stats.
type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	CallCount   int64  `json:"callCount"`
}
