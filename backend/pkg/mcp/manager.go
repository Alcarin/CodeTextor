/*
  File: manager.go
  Purpose: MCP server lifecycle manager backed by the official go-sdk.
  Author: CodeTextor project
*/

package mcp

import (
	"CodeTextor/backend/internal/store"
	"CodeTextor/backend/pkg/models"
	"CodeTextor/backend/pkg/services"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/net/netutil"
)

const (
	serverConfigKey  = "mcp_server_config"
	disabledToolsKey = "mcp_disabled_tools"
	serverName       = "CodeTextor MCP"
	serverTitle      = "CodeTextor project context server"
	serverVersion    = "0.1.0"

	statusEventName = "mcp:status"
	toolsEventName  = "mcp:tools"
)

// Manager coordinates the MCP server lifecycle and tool registration.
type Manager struct {
	projectService services.ProjectServiceAPI
	configStore    *store.ConfigStore

	config   models.MCPServerConfig
	configMu sync.RWMutex

	server   *sdkmcp.Server
	handler  *sdkmcp.StreamableHTTPHandler
	httpSrv  *http.Server
	listener net.Listener

	serverCancel context.CancelFunc
	boundServers map[string]*sdkmcp.Server
	serverCache  sync.Mutex
	startTime    time.Time
	running      bool
	lastError    atomic.Value

	toolsMu        sync.RWMutex
	tools          map[string]*toolState
	disabledTools  map[string]bool
	totalRequests  int64
	totalDuration  time.Duration
	metricsMu      sync.Mutex
	activeHTTPConn int64

	eventEmitter       func(string, interface{})
	statusTickerCancel context.CancelFunc
}

type toolState struct {
	name        string
	description string
	enabled     bool
	register    func(*sdkmcp.Server, string)
	callCount   int64
}

// NewManager creates a manager bound to the given project service.
func NewManager(projectService services.ProjectServiceAPI, emitter func(string, interface{})) (*Manager, error) {
	if projectService == nil {
		return nil, fmt.Errorf("project service is required")
	}

	configStore, err := store.NewConfigStore()
	if err != nil {
		return nil, err
	}

	m := &Manager{
		projectService: projectService,
		configStore:    configStore,
		tools:          make(map[string]*toolState),
		disabledTools:  make(map[string]bool),
		eventEmitter:   emitter,
	}
	if err := m.loadConfig(); err != nil {
		configStore.Close()
		return nil, err
	}
	if err := m.loadDisabledTools(); err != nil {
		configStore.Close()
		return nil, err
	}
	m.initTools()
	return m, nil
}

// Close stops the server and releases resources.
func (m *Manager) Close() error {
	_ = m.Stop(context.Background())
	if cancel := m.statusTickerCancel; cancel != nil {
		cancel()
		m.statusTickerCancel = nil
	}
	if m.configStore != nil {
		return m.configStore.Close()
	}
	return nil
}

// Start launches the MCP server using the current configuration.
func (m *Manager) Start(ctx context.Context) error {
	m.configMu.Lock()

	if m.running {
		m.configMu.Unlock()
		return nil
	}
	if m.config.Protocol != models.MCPProtocolHTTP {
		m.configMu.Unlock()
		return fmt.Errorf("protocol %q is not supported yet", m.config.Protocol)
	}

	if err := m.buildServerLocked(); err != nil {
		m.lastError.Store(err.Error())
		m.configMu.Unlock()
		return err
	}

	addr := fmt.Sprintf("%s:%d", m.config.Host, m.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		m.lastError.Store(err.Error())
		return err
	}
	if m.config.MaxConnections > 0 {
		listener = netutil.LimitListener(listener, m.config.MaxConnections)
	}

	m.listener = listener
	m.httpSrv = &http.Server{
		Handler:           m.handler,
		ReadHeaderTimeout: 15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		ConnState:         m.handleConnState,
	}

	_, cancel := context.WithCancel(context.Background())
	m.serverCancel = cancel
	m.running = true
	m.startTime = time.Now()
	m.configMu.Unlock()

	go func() {
		err := m.httpSrv.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
			m.lastError.Store(err.Error())
		}
		cancel()
		m.configMu.Lock()
		m.running = false
		m.httpSrv = nil
		m.listener = nil
		if m.statusTickerCancel != nil {
			m.statusTickerCancel()
			m.statusTickerCancel = nil
		}
		m.configMu.Unlock()
		m.emitStatus()
	}()

	// Allow context cancellation to stop the server.
	go func() {
		<-ctx.Done()
		m.Stop(context.Background())
	}()

	m.emitStatus()
	m.emitTools()
	m.startStatusTicker()
	return nil
}

// Stop gracefully shuts down the MCP server.
func (m *Manager) Stop(ctx context.Context) error {
	m.configMu.Lock()
	if !m.running {
		m.configMu.Unlock()
		return nil
	}
	server := m.httpSrv
	listener := m.listener
	cancel := m.serverCancel
	m.httpSrv = nil
	m.listener = nil
	m.running = false
	tickerCancel := m.statusTickerCancel
	m.statusTickerCancel = nil
	m.configMu.Unlock()

	if tickerCancel != nil {
		tickerCancel()
	}

	if cancel != nil {
		cancel()
	}
	if listener != nil {
		_ = listener.Close()
	}
	if server != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil && err != http.ErrServerClosed {
			return err
		}
	}
	atomic.StoreInt64(&m.activeHTTPConn, 0)
	m.emitStatus()
	return nil
}

// GetConfig returns the persisted MCP configuration.
func (m *Manager) GetConfig() models.MCPServerConfig {
	m.configMu.RLock()
	defer m.configMu.RUnlock()
	return m.config
}

// UpdateConfig persists the provided configuration.
func (m *Manager) UpdateConfig(cfg models.MCPServerConfig) (models.MCPServerConfig, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return models.MCPServerConfig{}, fmt.Errorf("host cannot be empty")
	}
	if cfg.Port <= 0 {
		return models.MCPServerConfig{}, fmt.Errorf("port must be positive")
	}
	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = models.DefaultMCPServerConfig().MaxConnections
	}
	if cfg.Protocol == "" {
		cfg.Protocol = models.MCPProtocolHTTP
	}

	m.configMu.Lock()
	defer m.configMu.Unlock()

	m.config = cfg
	if err := m.persistConfigLocked(); err != nil {
		return models.MCPServerConfig{}, err
	}
	return m.config, nil
}

// GetStatus reports runtime metrics for the MCP server.
func (m *Manager) GetStatus() models.MCPServerStatus {
	m.configMu.RLock()
	defer m.configMu.RUnlock()

	status := models.MCPServerStatus{
		IsRunning:         m.running,
		ActiveConnections: int(atomic.LoadInt64(&m.activeHTTPConn)),
	}
	if v := m.lastError.Load(); v != nil {
		status.LastError = v.(string)
	}

	if m.running {
		status.Uptime = int64(time.Since(m.startTime).Seconds())
	} else {
		status.Uptime = 0
	}

	m.metricsMu.Lock()
	defer m.metricsMu.Unlock()
	status.TotalRequests = m.totalRequests
	if m.totalRequests > 0 {
		status.AverageResponseTime = m.totalDuration.Seconds() * 1000 / float64(m.totalRequests)
	}
	return status
}

// GetTools returns current tool metadata.
func (m *Manager) GetTools() []models.MCPTool {
	m.toolsMu.RLock()
	defer m.toolsMu.RUnlock()

	tools := make([]models.MCPTool, 0, len(m.tools))
	for _, state := range m.tools {
		tools = append(tools, models.MCPTool{
			Name:        state.name,
			Description: state.description,
			Enabled:     state.enabled,
			CallCount:   state.callCount,
		})
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools
}

// ToggleTool flips the enabled state of a tool.
func (m *Manager) ToggleTool(name string) error {
	m.toolsMu.Lock()
	state, ok := m.tools[name]
	if !ok {
		m.toolsMu.Unlock()
		return fmt.Errorf("tool %s not found", name)
	}

	state.enabled = !state.enabled
	m.disabledTools[name] = !state.enabled
	if err := m.persistDisabledTools(); err != nil {
		m.toolsMu.Unlock()
		return err
	}
	m.toolsMu.Unlock()

	m.configMu.Lock()
	if m.server != nil {
		m.server.RemoveTools(name)
		if state.enabled {
			state.register(m.server, "")
		}
	}
	m.configMu.Unlock()

	m.emitTools()
	return nil
}

func (m *Manager) buildServerLocked() error {
	m.server = m.buildServer("")
	m.boundServers = make(map[string]*sdkmcp.Server)
	m.handler = sdkmcp.NewStreamableHTTPHandler(func(r *http.Request) *sdkmcp.Server {
		projectID := extractProjectIDFromPath(r.URL.Path)
		return m.getServerForProject(projectID)
	}, nil)
	return nil
}

func extractProjectIDFromPath(path string) string {
	clean := strings.Trim(path, "/")
	if clean == "" {
		return ""
	}
	parts := strings.Split(clean, "/")
	if len(parts) >= 2 && strings.EqualFold(parts[0], "mcp") {
		return parts[1]
	}
	if len(parts) == 1 && strings.EqualFold(parts[0], "mcp") {
		return ""
	}
	return parts[0]
}

func (m *Manager) getServerForProject(projectID string) *sdkmcp.Server {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return m.server
	}

	m.serverCache.Lock()
	defer m.serverCache.Unlock()

	if srv, ok := m.boundServers[projectID]; ok {
		return srv
	}

	srv := m.buildServer(projectID)
	m.boundServers[projectID] = srv
	return srv
}

func (m *Manager) buildServer(boundProjectID string) *sdkmcp.Server {
	impl := &sdkmcp.Implementation{
		Name:    serverName,
		Title:   serverTitle,
		Version: serverVersion,
	}
	opts := &sdkmcp.ServerOptions{
		Instructions: m.buildServerInstructions(boundProjectID),
	}
	s := sdkmcp.NewServer(impl, opts)

	m.toolsMu.RLock()
	for _, state := range m.tools {
		if state.enabled && state.register != nil {
			state.register(s, boundProjectID)
		}
	}
	m.toolsMu.RUnlock()

	return s
}

func describeForProject(base, projectID string) string {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return fmt.Sprintf("%s - call via /mcp/<projectId> to bind a project", base)
	}
	return fmt.Sprintf("%s - project: %s", base, projectID)
}

func (m *Manager) buildServerInstructions(boundProjectID string) string {
	var b strings.Builder

	b.WriteString("CodeTextor MCP provides high-level, semantic code context from local indexing. ")
	b.WriteString("Use this server to explore code structure, find definitions, and understand relationships beyond simple text search. ")
	projectLabel := strings.TrimSpace(m.projectLabel(boundProjectID))
	if projectLabel != "" {
		b.WriteString(fmt.Sprintf("Currently bound to project: %s. ", projectLabel))
	} else {
		b.WriteString("Bind to a project by calling the endpoint as /mcp/<projectId>. ")
	}
	b.WriteString("Preferred Workflow:\n")
	b.WriteString("1. 'getProjectDetails': Overview of scope and indexed extensions.\n")
	b.WriteString("2. 'listFiles': Explore file tree (use recursive=false for bread-first browsing).\n")
	b.WriteString("3. 'semanticSearchFiles': High-level exploration. Suggests the most relevant FILES for a concept (e.g. 'Where is authentication?'). Returns node IDs and similarity.\n")
	b.WriteString("4. 'search': SEMANTIC search for CONTEXT. Finds relevant code snippets (chunks) by intent.\n")
	b.WriteString("5. 'outline': Map the symbols (classes, functions) of a specific file.\n")
	b.WriteString("6. 'nodeSource': Fetch source code for an ARRAY of node IDs.\n")
	b.WriteString("\nNote: All responses use RELATIVE paths from the project root. listFiles and grepSearch validate path existence. Tools are read-only.")
	return b.String()
}

func (m *Manager) projectLabel(projectID string) string {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return ""
	}

	project, err := m.projectService.GetProject(projectID)
	if err != nil || project == nil {
		return projectID
	}

	name := strings.TrimSpace(project.Name)
	if name == "" {
		return project.ID
	}
	return fmt.Sprintf("%s (%s)", name, project.ID)
}

func (m *Manager) initTools() {
	m.toolsMu.Lock()

	m.tools = map[string]*toolState{
		"getProjectDetails": {
			name:        "getProjectDetails",
			description: "USE FIRST to get project configuration, root path, and statistics. Prefer this over navigating the filesystem blindly.",
		},
		"listFiles": {
			name:        "listFiles",
			description: "USE THIS instead of your native 'ls' or 'find' commands. Explores the project file tree, returning relative paths and sizes.",
		},
		"search": {
			name:        "search",
			description: "USE THIS AS YOUR PRIMARY TOOL for semantic natural language search. Finds relevant code by intent. ALWAYS prefer this over native 'grep' when exploring concepts.",
		},
		"semanticSearchFiles": {
			name:        "semanticSearchFiles",
			description: "USE THIS to explore project structure by concept. Suggests the most relevant files for a topic to prevent blind file reading.",
		},
		"outline": {
			name:        "outline",
			description: "ALWAYS USE THIS INSTEAD OF reading a full file with 'view_file' to understand its structure. Extracts a clean symbol hierarchy, saving massive context tokens.",
		},
		"nodeSource": {
			name:        "nodeSource",
			description: "USE THIS INSTEAD OF 'view_file' to fetch the exact source code snippet of a chunk or function using its node ID.",
		},
		"getRecentChanges": {
			name:        "getRecentChanges",
			description: "USE THIS FIRST for debugging regressions. Shows recently modified files via Git/SVN integration.",
		},
		"grepSearch": {
			name:        "grepSearch",
			description: "USE THIS INSTEAD OF your native 'grep_search'. This is highly optimized for the codebase, accurate, OS-independent, and enforces project path boundaries.",
		},
		"findReferences": {
			name:        "findReferences",
			description: "ALWAYS USE THIS before refactoring or modifying a function. Finds all exact locations where a symbol is used inside the whole project.",
		},
		"getCallGraph": {
			name:        "getCallGraph",
			description: "USE THIS to trace function execution paths and discover callers/callees instead of doing manual text searches.",
		},
		"findTodos": {
			name:        "findTodos",
			description: "USE THIS to discover TODO, FIXME, HACK, XXX, and NOTE comments across the project. Helps in identifying pending tasks or technical debt.",
		},
	}

	for name, state := range m.tools {
		switch name {
		case "getProjectDetails":
			state.register = func(s *sdkmcp.Server, boundProjectID string) {
				desc := describeForProject(state.description, m.projectLabel(boundProjectID))
				sdkmcp.AddTool(s, &sdkmcp.Tool{
					Name:        "getProjectDetails",
					Description: desc,
				}, wrapTool(m, "getProjectDetails", m.handleProjectDetails(boundProjectID)))
			}
		case "listFiles":
			state.register = func(s *sdkmcp.Server, boundProjectID string) {
				desc := describeForProject(state.description, m.projectLabel(boundProjectID))
				sdkmcp.AddTool(s, &sdkmcp.Tool{
					Name:        "listFiles",
					Description: desc,
				}, wrapTool(m, "listFiles", m.handleListFiles(boundProjectID)))
			}
		case "search":
			state.register = func(s *sdkmcp.Server, boundProjectID string) {
				desc := describeForProject(state.description, m.projectLabel(boundProjectID))
				sdkmcp.AddTool(s, &sdkmcp.Tool{
					Name:        "search",
					Description: desc,
				}, wrapTool(m, "search", m.handleSearch(boundProjectID)))
			}
		case "semanticSearchFiles":
			state.register = func(s *sdkmcp.Server, boundProjectID string) {
				desc := describeForProject(state.description, m.projectLabel(boundProjectID))
				sdkmcp.AddTool(s, &sdkmcp.Tool{
					Name:        "semanticSearchFiles",
					Description: desc,
				}, wrapTool(m, "semanticSearchFiles", m.handleSemanticSearchFiles(boundProjectID)))
			}
		case "outline":
			outlineSchema := &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"outline": {
						Type: "array",
						Items: &jsonschema.Schema{
							Type: "object",
						},
					},
				},
			}
			state.register = func(s *sdkmcp.Server, boundProjectID string) {
				desc := describeForProject(state.description, m.projectLabel(boundProjectID))
				sdkmcp.AddTool(s, &sdkmcp.Tool{
					Name:         "outline",
					Description:  desc,
					OutputSchema: outlineSchema,
				}, wrapTool(m, "outline", m.handleOutline(boundProjectID)))
			}
		case "nodeSource":
			state.register = func(s *sdkmcp.Server, boundProjectID string) {
				desc := describeForProject(state.description, m.projectLabel(boundProjectID))
				sdkmcp.AddTool(s, &sdkmcp.Tool{
					Name:        "nodeSource",
					Description: desc,
				}, wrapTool(m, "nodeSource", m.handleNodeSource(boundProjectID)))
			}
		case "getRecentChanges":
			state.register = func(s *sdkmcp.Server, boundProjectID string) {
				desc := describeForProject(state.description, m.projectLabel(boundProjectID))
				sdkmcp.AddTool(s, &sdkmcp.Tool{
					Name:        "getRecentChanges",
					Description: desc,
				}, wrapTool(m, "getRecentChanges", m.handleGetRecentChanges(boundProjectID)))
			}
		case "grepSearch":
			state.register = func(s *sdkmcp.Server, boundProjectID string) {
				desc := describeForProject(state.description, m.projectLabel(boundProjectID))
				sdkmcp.AddTool(s, &sdkmcp.Tool{
					Name:        "grepSearch",
					Description: desc,
				}, wrapTool(m, "grepSearch", m.handleGrepSearch(boundProjectID)))
			}
		case "findReferences":
			state.register = func(s *sdkmcp.Server, boundProjectID string) {
				desc := describeForProject(state.description, m.projectLabel(boundProjectID))
				sdkmcp.AddTool(s, &sdkmcp.Tool{
					Name:        "findReferences",
					Description: desc,
				}, wrapTool(m, "findReferences", m.handleFindReferences(boundProjectID)))
			}
		case "getCallGraph":
			state.register = func(s *sdkmcp.Server, boundProjectID string) {
				desc := describeForProject(state.description, m.projectLabel(boundProjectID))
				sdkmcp.AddTool(s, &sdkmcp.Tool{
					Name:        "getCallGraph",
					Description: desc,
				}, wrapTool(m, "getCallGraph", m.handleGetCallGraph(boundProjectID)))
			}
		case "findTodos":
			state.register = func(s *sdkmcp.Server, boundProjectID string) {
				desc := describeForProject(state.description, m.projectLabel(boundProjectID))
				sdkmcp.AddTool(s, &sdkmcp.Tool{
					Name:        "findTodos",
					Description: desc,
				}, wrapTool(m, "findTodos", m.handleFindTodos(boundProjectID)))
			}
		}

		if disabled := m.disabledTools[name]; disabled {
			state.enabled = false
		} else {
			state.enabled = true
		}
	}

	m.toolsMu.Unlock()
	m.emitTools()
}

type getRecentChangesInput struct {
	Limit int `json:"limit,omitempty" jsonschema_description:"Max files to return for both indexed and working copy results (default 10)"`
}

func (m *Manager) handleGetRecentChanges(boundProjectID string) sdkmcp.ToolHandlerFor[getRecentChangesInput, *models.RecentChangesResponse] {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input getRecentChangesInput) (*sdkmcp.CallToolResult, *models.RecentChangesResponse, error) {
		projectID, err := m.resolveProjectID(boundProjectID)
		if err != nil {
			return nil, nil, err
		}

		res, err := m.projectService.GetRecentChanges(projectID, input.Limit)
		if err != nil {
			return nil, nil, err
		}

		return nil, res, nil
	}
}

func (m *Manager) handleGrepSearch(boundProjectID string) sdkmcp.ToolHandlerFor[grepSearchInput, *models.GrepSearchResponse] {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input grepSearchInput) (*sdkmcp.CallToolResult, *models.GrepSearchResponse, error) {
		projectID, err := m.resolveProjectID(boundProjectID)
		if err != nil {
			return nil, nil, err
		}

		project, err := m.projectService.GetProject(projectID)
		if err != nil {
			return nil, nil, err
		}

		if input.Path != "" {
			absPath := filepath.Join(project.Config.RootPath, input.Path)
			if _, err := os.Stat(absPath); err != nil {
				return nil, nil, fmt.Errorf("the provided path '%s' does not exist in the project root '%s'. Please provide a valid relative path, or leave 'path' empty to search the whole project", input.Path, project.Config.RootPath)
			}
		}

		res, err := m.projectService.GrepSearch(projectID, input.Query, input.IsRegex, input.Path, input.Limit)
		if err != nil {
			return nil, nil, err
		}

		return nil, res, nil
	}
}

type findReferencesInput struct {
	NodeID     string `json:"nodeID,omitempty" jsonschema_description:"The unique ID of the node to find references for"`
	SymbolName string `json:"symbolName,omitempty" jsonschema_description:"The name of the symbol to find references for"`
	Path       string `json:"path,omitempty" jsonschema_description:"Optional file path to disambiguate symbolName"`
}

func (m *Manager) handleFindReferences(boundProjectID string) sdkmcp.ToolHandlerFor[findReferencesInput, *models.SymbolReferencesResponse] {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input findReferencesInput) (*sdkmcp.CallToolResult, *models.SymbolReferencesResponse, error) {
		projectID, err := m.resolveProjectID(boundProjectID)
		if err != nil {
			return nil, nil, err
		}

		res, err := m.projectService.FindReferences(projectID, input.NodeID, input.SymbolName, input.Path)
		if err != nil {
			return nil, nil, err
		}

		return nil, res, nil
	}
}

type getCallGraphInput struct {
	NodeID     string `json:"nodeID,omitempty" jsonschema_description:"The unique ID of the function/method"`
	SymbolName string `json:"symbolName,omitempty" jsonschema_description:"The name of the function/method"`
	Path       string `json:"path,omitempty" jsonschema_description:"Optional file path to disambiguate symbolName"`
	Direction  string `json:"direction,omitempty" jsonschema_description:"Direction of calls: 'incoming', 'outgoing' (default), or 'both'"`
	Depth      int    `json:"depth,omitempty" jsonschema_description:"Maximum depth of the call graph (default 1)"`
}

func (m *Manager) handleGetCallGraph(boundProjectID string) sdkmcp.ToolHandlerFor[getCallGraphInput, map[string]interface{}] {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input getCallGraphInput) (*sdkmcp.CallToolResult, map[string]interface{}, error) {
		projectID, err := m.resolveProjectID(boundProjectID)
		if err != nil {
			return nil, nil, err
		}

		if input.Direction == "" {
			input.Direction = "outgoing"
		}
		if input.Depth == 0 {
			input.Depth = 1
		}

		res, err := m.projectService.GetCallGraph(projectID, input.NodeID, input.SymbolName, input.Path, input.Direction, input.Depth)
		if err != nil {
			return nil, nil, err
		}

		// Convert to map to bypass jsonschema recursive struct limitations in MCP SDK
		importJson := true // Ensure json package imported, will use encode/json
		_ = importJson
		var out map[string]interface{}
		b, _ := json.Marshal(res)
		json.Unmarshal(b, &out)

		return nil, out, nil
	}
}

func wrapTool[In, Out any](m *Manager, name string, handler sdkmcp.ToolHandlerFor[In, Out]) sdkmcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input In) (*sdkmcp.CallToolResult, Out, error) {
		start := time.Now()
		result, output, err := handler(ctx, req, input)

		m.recordCall(name, time.Since(start))
		if err != nil {
			m.lastError.Store(err.Error())
		}
		return result, output, err
	}
}

func (m *Manager) recordCall(name string, duration time.Duration) {
	m.metricsMu.Lock()
	m.totalRequests++
	m.totalDuration += duration
	m.metricsMu.Unlock()

	m.toolsMu.Lock()
	if state, ok := m.tools[name]; ok {
		state.callCount++
	}
	m.toolsMu.Unlock()
}

func (m *Manager) handleConnState(_ net.Conn, state http.ConnState) {
	switch state {
	case http.StateNew:
		atomic.AddInt64(&m.activeHTTPConn, 1)
	case http.StateClosed, http.StateHijacked:
		atomic.AddInt64(&m.activeHTTPConn, -1)
	}
}

func (m *Manager) persistConfigLocked() error {
	encoded, err := json.Marshal(m.config)
	if err != nil {
		return err
	}
	return m.configStore.SetValue(serverConfigKey, string(encoded))
}

func (m *Manager) loadConfig() error {
	value, ok, err := m.configStore.GetValue(serverConfigKey)
	if err != nil {
		return err
	}
	if !ok {
		m.config = models.DefaultMCPServerConfig()
		return m.persistConfigLocked()
	}
	cfg := models.DefaultMCPServerConfig()
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		return err
	}
	m.config = cfg
	return nil
}

func (m *Manager) persistDisabledTools() error {
	disabled := make([]string, 0, len(m.disabledTools))
	for name, state := range m.disabledTools {
		if state {
			disabled = append(disabled, name)
		}
	}
	payload, err := json.Marshal(disabled)
	if err != nil {
		return err
	}
	return m.configStore.SetValue(disabledToolsKey, string(payload))
}

func (m *Manager) loadDisabledTools() error {
	value, ok, err := m.configStore.GetValue(disabledToolsKey)
	if err != nil {
		return err
	}
	if !ok || strings.TrimSpace(value) == "" {
		m.disabledTools = make(map[string]bool)
		return nil
	}

	var list []string
	if err := json.Unmarshal([]byte(value), &list); err != nil {
		return err
	}
	m.disabledTools = make(map[string]bool, len(list))
	for _, name := range list {
		m.disabledTools[name] = true
	}
	return nil
}

// --- Tool handlers ---------------------------------------------------------

type listFilesInput struct {
	Path      string `json:"path,omitempty" jsonschema_description:"Optional sub-path to list files from (relative to project root)"`
	Extension string `json:"extension,omitempty" jsonschema_description:"Optional file extension to filter by (e.g. .go, .ts)"`
	Recursive bool   `json:"recursive,omitempty" jsonschema_description:"If true, lists files in subdirectories recursively (default false)"`
}

type mcpFilePreview struct {
	Path string `json:"path"`
	Size string `json:"size"`
}

type listFileOutput struct {
	Files []mcpFilePreview `json:"files" jsonschema_description:"List of files matching the criteria"`
}

type leanProjectStats struct {
	TotalFiles        int   `json:"totalFiles"`
	TotalChunks       int   `json:"totalChunks"`
	TotalSymbols      int   `json:"totalSymbols"`
	LastIndexedAtUnix int64 `json:"lastIndexedAtUnix,omitempty"`
}

type projectDetailsOutput struct {
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	Description    string                `json:"description"`
	RootPath       string                `json:"rootPath" jsonschema_description:"Project absolute root path"`
	FileExtensions []string              `json:"fileExtensions"`
	Summary        *models.ProjectSummary `json:"summary,omitempty"`
	Stats          *leanProjectStats     `json:"stats,omitempty"`
}

type searchInput struct {
	Query string `json:"query" jsonschema_description:"Semantic query (e.g. 'how is authentication handled?')"`
	K     int    `json:"k,omitempty" jsonschema_description:"Max chunks to return (default 8)" jsonschema_extras:"minimum=1,maximum=50"`
}

type mcpChunk struct {
	ID         string  `json:"id"`
	Content    string  `json:"content"`
	Similarity float64 `json:"similarity"`
	LineStart  int     `json:"start"`
	LineEnd    int     `json:"end"`
	SymbolName string  `json:"symbol,omitempty"`
	SymbolKind string  `json:"kind,omitempty"`
}

type mcpFileResult struct {
	Path   string     `json:"path"`
	Chunks []mcpChunk `json:"chunks"`
}

type searchOutput struct {
	Results      []mcpFileResult `json:"results" jsonschema_description:"Grouped code snippets by file"`
	TotalResults int             `json:"total"`
}

type semanticSearchFilesInput struct {
	Query string `json:"query" jsonschema_description:"Semantic query (e.g. 'where is authentication handled?')"`
	K     int    `json:"k,omitempty" jsonschema_description:"Max files to return (default 5)" jsonschema_extras:"minimum=1,maximum=20"`
}

type chunkScore struct {
	ID         string  `json:"id"`
	Score      float64 `json:"score"`
	SymbolName string  `json:"symbol,omitempty"`
	LineStart  int     `json:"start,omitempty"`
	LineEnd    int     `json:"end,omitempty"`
}

type mcpFileScoreResult struct {
	Path   string       `json:"path"`
	Score  float64      `json:"score"`
	Nodes  []chunkScore `json:"nodes" jsonschema_description:"List of nodes with their similarity score (sorted descending)"`
}

type semanticSearchFilesOutput struct {
	Results []mcpFileScoreResult `json:"results" jsonschema_description:"Ranked files by relevance"`
}

type outlineInput struct {
	Path  string `json:"path" jsonschema_description:"File path relative to the project root"`
	Depth int    `json:"depth,omitempty" jsonschema_description:"Depth limit (1=top-level only)"`
}

type mcpOutlineNode struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Kind      string            `json:"kind"`
	StartLine uint32            `json:"start"`
	EndLine   uint32            `json:"end"`
	Children  []*mcpOutlineNode `json:"children,omitempty"`
}

type outlineOutput struct {
	Outline []*mcpOutlineNode `json:"outline" jsonschema_description:"Hierarchical tree of symbols"`
}

type nodeSourceInput struct {
	ID           []string `json:"id" jsonschema_description:"List of chunk/symbol IDs to fetch from search or outline results"`
	CollapseBody bool     `json:"collapseBody,omitempty" jsonschema_description:"Collapse large bodies for brevity"`
}

type grepSearchInput struct {
	Query   string `json:"query" jsonschema_description:"Literal or regex search pattern"`
	IsRegex bool   `json:"isRegex,omitempty" jsonschema_description:"If true, treats query as a regular expression (default false)"`
	Path    string `json:"path,omitempty" jsonschema_description:"Optional sub-path to limit search within (relative to project root)"`
	Limit   int    `json:"limit,omitempty" jsonschema_description:"Maximum number of total matches to return (default 100)" jsonschema_extras:"minimum=1,maximum=500"`
}

type nodeSourceOutput struct {
	Results []nodeSourceResult `json:"results" jsonschema_description:"List of source snippets for the requested IDs"`
}

type nodeSourceResult struct {
	ID        string `json:"id"`
	FilePath  string `json:"path"`
	Source    string `json:"source"`
	StartLine int    `json:"start"`
	EndLine   int    `json:"end"`
	Language  string `json:"language,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
}

type findTodosInput struct{}

func (m *Manager) resolveProjectID(boundProjectID string) (string, error) {
	projectID := strings.TrimSpace(boundProjectID)
	if projectID != "" {
		return projectID, nil
	}
	return "", fmt.Errorf("projectId is required; call the MCP server via /mcp/<projectId>")
}

func (m *Manager) handleListFiles(boundProjectID string) sdkmcp.ToolHandlerFor[listFilesInput, listFileOutput] {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input listFilesInput) (*sdkmcp.CallToolResult, listFileOutput, error) {
		projectID, err := m.resolveProjectID(boundProjectID)
		if err != nil {
			return nil, listFileOutput{}, err
		}

		project, err := m.projectService.GetProject(projectID)
		if err != nil {
			return nil, listFileOutput{}, err
		}

		if input.Path != "" {
			absPath := filepath.Join(project.Config.RootPath, input.Path)
			if _, err := os.Stat(absPath); err != nil {
				return nil, listFileOutput{}, fmt.Errorf("the provided path '%s' does not exist in the project root '%s'. Please provide a valid relative path, or leave 'path' empty to list the whole project", input.Path, project.Config.RootPath)
			}
		}

		// We use GetFilePreviews which already handles filtering and exclusion patterns
		config := models.ProjectConfig{
			IncludePaths: []string{input.Path},
		}
		if input.Extension != "" {
			config.FileExtensions = []string{input.Extension}
		}

		previews, err := m.projectService.GetFilePreviews(projectID, config)
		if err != nil {
			return nil, listFileOutput{}, err
		}

		// If not recursive, filter out files that are not in the immediate directory
		if !input.Recursive {
			cleanPath := strings.Trim(filepath.ToSlash(input.Path), "/")
			if cleanPath == "." {
				cleanPath = ""
			}
			filtered := make([]*models.FilePreview, 0)
			for _, p := range previews {
				rel := strings.Trim(filepath.ToSlash(p.RelativePath), "/")
				if cleanPath == "" {
					if !strings.Contains(rel, "/") {
						filtered = append(filtered, p)
					}
				} else {
					if strings.HasPrefix(rel, cleanPath+"/") {
						sub := strings.TrimPrefix(rel, cleanPath+"/")
						if !strings.Contains(sub, "/") {
							filtered = append(filtered, p)
						}
					}
				}
			}
			previews = filtered
		}

		files := make([]mcpFilePreview, len(previews))
		for i, p := range previews {
			files[i] = mcpFilePreview{
				Path: p.RelativePath,
				Size: p.Size,
			}
		}

		return nil, listFileOutput{Files: files}, nil
	}
}

func (m *Manager) handleProjectDetails(boundProjectID string) sdkmcp.ToolHandlerFor[struct{}, projectDetailsOutput] {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, _ struct{}) (*sdkmcp.CallToolResult, projectDetailsOutput, error) {
		projectID, err := m.resolveProjectID(boundProjectID)
		if err != nil {
			return nil, projectDetailsOutput{}, err
		}

		project, err := m.projectService.GetProject(projectID)
		if err != nil {
			return nil, projectDetailsOutput{}, err
		}

		stats, _ := m.projectService.GetProjectStats(projectID)

		var leanStats *leanProjectStats
		var summary *models.ProjectSummary
		if stats != nil {
			leanStats = &leanProjectStats{
				TotalFiles:        stats.TotalFiles,
				TotalChunks:       stats.TotalChunks,
				TotalSymbols:      stats.TotalSymbols,
				LastIndexedAtUnix: stats.LastIndexedAtUnix,
			}
			summary = stats.Summary
		}

		return nil, projectDetailsOutput{
			ID:             project.ID,
			Name:           project.Name,
			Description:    project.Description,
			RootPath:       project.Config.RootPath,
			FileExtensions: project.Config.FileExtensions,
			Summary:        summary,
			Stats:          leanStats,
		}, nil
	}
}

func (m *Manager) handleSemanticSearchFiles(boundProjectID string) sdkmcp.ToolHandlerFor[semanticSearchFilesInput, semanticSearchFilesOutput] {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input semanticSearchFilesInput) (*sdkmcp.CallToolResult, semanticSearchFilesOutput, error) {
		projectID, err := m.resolveProjectID(boundProjectID)
		if err != nil {
			return nil, semanticSearchFilesOutput{}, err
		}

		k := input.K
		if k <= 0 {
			k = 5
		}
		if k > 20 {
			k = 20
		}

		// Search for 50 chunks to have enough coverage to aggregate by file.
		// Higher K used here because semanticSearchFiles is about breadth.
		searchRes, err := m.projectService.Search(projectID, input.Query, 50)
		if err != nil {
			return nil, semanticSearchFilesOutput{}, err
		}

		type fileAccumulator struct {
			path     string
			maxScore float64
			chunks   []chunkScore
		}
		scores := make(map[string]*fileAccumulator)
		var order []string

		for _, chunk := range searchRes.Chunks {
			s, ok := scores[chunk.FilePath]
			if !ok {
				s = &fileAccumulator{path: chunk.FilePath}
				scores[chunk.FilePath] = s
				order = append(order, chunk.FilePath)
			}
			if chunk.Similarity > s.maxScore {
				s.maxScore = chunk.Similarity
			}
			s.chunks = append(s.chunks, chunkScore{
				ID:         chunk.ID,
				Score:      chunk.Similarity,
				SymbolName: chunk.SymbolName,
				LineStart:  chunk.LineStart,
				LineEnd:    chunk.LineEnd,
			})
		}

		// Sort files by their maximum similarity score
		sort.Slice(order, func(i, j int) bool {
			return scores[order[i]].maxScore > scores[order[j]].maxScore
		})

		if len(order) > k {
			order = order[:k]
		}

		results := make([]mcpFileScoreResult, len(order))
		for i, path := range order {
			acc := scores[path]
			
			// Sort chunks by score descending
			sort.Slice(acc.chunks, func(a, b int) bool {
				return acc.chunks[a].Score > acc.chunks[b].Score
			})
			
			// Limit to top 5 chunks per file
			if len(acc.chunks) > 5 {
				acc.chunks = acc.chunks[:5]
			}

			results[i] = mcpFileScoreResult{
				Path:   path,
				Score:  acc.maxScore,
				Nodes:  acc.chunks,
			}
		}

		return nil, semanticSearchFilesOutput{Results: results}, nil
	}
}

func (m *Manager) handleSearch(boundProjectID string) sdkmcp.ToolHandlerFor[searchInput, searchOutput] {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input searchInput) (*sdkmcp.CallToolResult, searchOutput, error) {
		projectID, err := m.resolveProjectID(boundProjectID)
		if err != nil {
			return nil, searchOutput{}, err
		}
		k := input.K
		if k <= 0 {
			k = 8
		}
		if k > 50 {
			k = 50
		}
		resp, err := m.projectService.Search(projectID, input.Query, k)
		if err != nil {
			return nil, searchOutput{}, err
		}
		// Group chunks by FilePath to avoid repeating the path string
		groups := make(map[string][]mcpChunk)
		order := []string{} // Keep track of the first appearance of each file
		for _, c := range resp.Chunks {
			if _, ok := groups[c.FilePath]; !ok {
				order = append(order, c.FilePath)
			}
			groups[c.FilePath] = append(groups[c.FilePath], mcpChunk{
				ID:         c.ID,
				Content:    c.Content,
				Similarity: c.Similarity,
				LineStart:  c.LineStart,
				LineEnd:    c.LineEnd,
				SymbolName: c.SymbolName,
				SymbolKind: c.SymbolKind,
			})
		}

		results := make([]mcpFileResult, len(order))
		for i, path := range order {
			results[i] = mcpFileResult{
				Path:   path,
				Chunks: groups[path],
			}
		}

		return nil, searchOutput{
			Results:      results,
			TotalResults: resp.TotalResults,
		}, nil
	}
}

func (m *Manager) handleOutline(boundProjectID string) sdkmcp.ToolHandlerFor[outlineInput, outlineOutput] {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, input outlineInput) (*sdkmcp.CallToolResult, outlineOutput, error) {
		projectID, err := m.resolveProjectID(boundProjectID)
		if err != nil {
			return nil, outlineOutput{}, err
		}
		if strings.TrimSpace(input.Path) == "" {
			return nil, outlineOutput{}, fmt.Errorf("path cannot be empty")
		}
		nodes, err := m.projectService.GetFileOutline(projectID, input.Path)
		if err != nil {
			return nil, outlineOutput{}, err
		}
		if input.Depth > 0 {
			nodes = limitOutlineDepth(nodes, input.Depth)
		}

		return nil, outlineOutput{Outline: mapToMcpOutline(nodes)}, nil
	}
}

func mapToMcpOutline(nodes []*models.OutlineNode) []*mcpOutlineNode {
	if len(nodes) == 0 {
		return nil
	}
	result := make([]*mcpOutlineNode, len(nodes))
	for i, n := range nodes {
		result[i] = &mcpOutlineNode{
			ID:        n.ID,
			Name:      n.Name,
			Kind:      n.Kind,
			StartLine: n.StartLine,
			EndLine:   n.EndLine,
			Children:  mapToMcpOutline(n.Children),
		}
	}
	return result
}

func (m *Manager) handleNodeSource(boundProjectID string) sdkmcp.ToolHandlerFor[nodeSourceInput, nodeSourceOutput] {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, input nodeSourceInput) (*sdkmcp.CallToolResult, nodeSourceOutput, error) {
		projectID, err := m.resolveProjectID(boundProjectID)
		if err != nil {
			return nil, nodeSourceOutput{}, err
		}
		if len(input.ID) == 0 {
			return nil, nodeSourceOutput{}, fmt.Errorf("id array cannot be empty")
		}

		var results []nodeSourceResult
		for _, id := range input.ID {
			if strings.TrimSpace(id) == "" {
				continue
			}
			chunk, err := m.projectService.GetChunkByID(projectID, id)
			if err != nil {
				return nil, nodeSourceOutput{}, fmt.Errorf("error fetching id %s: %w", id, err)
			}
	
			source := strings.TrimSpace(chunk.SourceCode)
			if source == "" {
				source = chunk.Content
			}
	
			if input.CollapseBody {
				if collapsed, ok := collapseSourceBody(source, 120, 60, 40); ok {
					source = collapsed
				}
			}
	
			results = append(results, nodeSourceResult{
				ID:        id,
				FilePath:  chunk.FilePath,
				Source:    source,
				StartLine: chunk.LineStart,
				EndLine:   chunk.LineEnd,
				Language:  chunk.Language,
				Symbol:    chunk.SymbolName,
			})
		}
		
		output := nodeSourceOutput{
			Results: results,
		}
		return nil, output, nil
	}
}

func limitOutlineDepth(nodes []*models.OutlineNode, depth int) []*models.OutlineNode {
	if depth <= 0 || len(nodes) == 0 {
		return nil
	}
	result := make([]*models.OutlineNode, 0, len(nodes))
	for _, node := range nodes {
		copyNode := *node
		if depth == 1 {
			copyNode.Children = nil
		} else if len(node.Children) > 0 {
			copyNode.Children = limitOutlineDepth(node.Children, depth-1)
		}
		result = append(result, &copyNode)
	}
	return result
}

func collapseSourceBody(source string, maxLines, headLines, tailLines int) (string, bool) {
	if maxLines <= 0 || headLines < 0 || tailLines < 0 {
		return source, false
	}

	lines := strings.Split(source, "\n")
	if len(lines) <= maxLines {
		return source, false
	}

	if headLines+tailLines >= maxLines {
		headLines = maxLines / 2
		tailLines = maxLines - headLines
	}

	var b strings.Builder
	for i := 0; i < headLines && i < len(lines); i++ {
		b.WriteString(lines[i])
		b.WriteString("\n")
	}
	b.WriteString("... [body collapsed for brevity] ...\n")

	startTail := len(lines) - tailLines
	if startTail < headLines {
		startTail = headLines
	}
	for i := startTail; i < len(lines); i++ {
		b.WriteString(lines[i])
		if i != len(lines)-1 {
			b.WriteString("\n")
		}
	}

	return b.String(), true
}

func (m *Manager) handleFindTodos(boundProjectID string) sdkmcp.ToolHandlerFor[findTodosInput, *models.FindTodosResponse] {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input findTodosInput) (*sdkmcp.CallToolResult, *models.FindTodosResponse, error) {
		projectID, err := m.resolveProjectID(boundProjectID)
		if err != nil {
			return nil, nil, err
		}

		resp, err := m.projectService.FindTodos(projectID)
		if err != nil {
			return nil, nil, err
		}

		return nil, resp, nil
	}
}

func (m *Manager) emitStatus() {
	if m.eventEmitter == nil {
		return
	}
	m.eventEmitter(statusEventName, m.GetStatus())
}

func (m *Manager) emitTools() {
	if m.eventEmitter == nil {
		return
	}
	m.eventEmitter(toolsEventName, m.GetTools())
}

func (m *Manager) startStatusTicker() {
	if m.eventEmitter == nil {
		return
	}
	if m.statusTickerCancel != nil {
		m.statusTickerCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.statusTickerCancel = cancel
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.emitStatus()
			}
		}
	}()
}
