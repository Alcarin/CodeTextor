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

	b.WriteString("CodeTextor MCP is your primary gateway to the project. It provides high-level, semantic code context from local indexing. ")
	b.WriteString("MANDATORY: ALWAYS prefer these specialized tools over generic filesystem or grep commands. They are optimized for speed, token efficiency, and accuracy. ")
	projectLabel := strings.TrimSpace(m.projectLabel(boundProjectID))
	if projectLabel != "" {
		b.WriteString(fmt.Sprintf("Currently bound to project: %s. ", projectLabel))
	} else {
		b.WriteString("Bind to a project by calling the endpoint as /mcp/<projectId>. ")
	}
	b.WriteString("\nPreferred Workflow:\n")
	b.WriteString("1. 'getProjectDetails': CALL THIS FIRST to understand the project scope, main languages, and entry points.\n")
	b.WriteString("2. 'listFiles': USE THIS for exploration. It returns tabular data (Size, Lines, Symbols) and supports 'depth' and 'extension' filters.\n")
	b.WriteString("3. 'semanticSearchFiles': CONCEPTUAL EXPLORATION. Use this to find which FILES are relevant to a concept (e.g., 'auth logic') before reading code.\n")
	b.WriteString("4. 'search': SEMANTIC SNIPPET SEARCH. Finds the most relevant code chunks by intent. Use for specific 'how-to' questions.\n")
	b.WriteString("5. 'outline': STRUCTURAL ANALYSIS. ALWAYS use this instead of reading full files to map symbols (classes, methods).\n")
	b.WriteString("6. 'nodeSource': SELECTIVE SOURCE FETCH. Fetch specific code snippets using IDs from search/outline. Supports FUZZY IDs ('path|name' or 'path|Lstart-end') to quickly jump to definitions.\n")
	b.WriteString("7. STATIC ANALYSIS ('findReferences', 'getCallGraph', etc.): MANDATORY before any refactoring to prevent regressions and ensure code quality.\n")
	b.WriteString("\nNote: All paths are RELATIVE to the project root. Tools are read-only and enforce project boundaries.")
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
			description: "MANDATORY INITIAL STEP. Fetch project configuration, root path, statistics, and structural summary. Use this to orient yourself before navigating.",
		},
		"listFiles": {
			name:        "listFiles",
			description: "PRIMARY EXPLORATION TOOL. USE INSTEAD OF 'ls' or 'find'. Returns relative paths, sizes, line counts, and symbol density. Supports extension and depth filtering.",
		},
		"search": {
			name:        "search",
			description: "SEMANTIC CODE SEARCH. Find relevant code snippets (chunks) by natural language intent. ALWAYS prefer this over native grep for conceptual queries.",
		},
		"semanticSearchFiles": {
			name:        "semanticSearchFiles",
			description: "STRUCTURAL DISCOVERY. Identify which files are most relevant to a concept or feature without blind reading. Returns ranks and key node IDs.",
		},
		"outline": {
			name:        "outline",
			description: "SYMBOL MAPPING. ALWAYS USE THIS instead of reading a full file to understand its internal structure, classes, and methods. Extremely token-efficient.",
		},
		"nodeSource": {
			name:        "nodeSource",
			description: "TARGETED SOURCE FETCH. Retrieval tool for specific chunks/symbols. Supports FUZZY IDs (e.g., 'path|symbolName'). USE INSTEAD OF 'view_file' for surgical reads.",
		},
		"getRecentChanges": {
			name:        "getRecentChanges",
			description: "REGRESSION TRACKER. View recently modified files in the working copy or indexing history. Essential for debugging latest changes.",
		},
		"grepSearch": {
			name:        "grepSearch",
			description: "OS-AGNOSTIC TEXT SEARCH. Use for literal or regex matches. Highly optimized and returns results in a compact tabular format [File, Line, Content].",
		},
		"findReferences": {
			name:        "findReferences",
			description: "IMPACT ANALYSIS. MANDATORY before modifications. Finds all exact call-site references of a symbol across the whole project to avoid regressions.",
		},
		"findImplementations": {
			name:        "findImplementations",
			description: "OOP EXPLORATION. Finds all classes, interfaces, or traits that implement or extend a specific target. Crucial for understanding polymorphic logic.",
		},
		"getCallGraph": {
			name:        "getCallGraph",
			description: "ARCHITECTURAL TRACING. Generates a tree of function callers/callees. Fundamental for understanding flow and ensuring quality refactoring.",
		},
		"findTodos": {
			name:        "findTodos",
			description: "DEBT DISCOVERY. Locates all TODO, FIXME, HACK, and NOTE comments. Returns a structured map by category with symbol IDs and global statistics.",
		},
		"getPackageGraph": {
			name:        "getPackageGraph",
			description: "DEPENDENCY OVERVIEW. Map high-level coupling between packages and external libraries. Essential for architectural decisions.",
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
		case "findImplementations":
			state.register = func(s *sdkmcp.Server, boundProjectID string) {
				desc := describeForProject(state.description, m.projectLabel(boundProjectID))
				sdkmcp.AddTool(s, &sdkmcp.Tool{
					Name:        "findImplementations",
					Description: desc,
				}, wrapTool(m, "findImplementations", m.handleFindImplementations(boundProjectID)))
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
		case "getPackageGraph":
			state.register = func(s *sdkmcp.Server, boundProjectID string) {
				desc := describeForProject(state.description, m.projectLabel(boundProjectID))
				sdkmcp.AddTool(s, &sdkmcp.Tool{
					Name:        "getPackageGraph",
					Description: desc,
				}, wrapTool(m, "getPackageGraph", m.handleGetPackageGraph(boundProjectID)))
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
	Limit int `json:"limit,omitempty" jsonschema_description:"Max files to return for both indexed and working copy results (default 10)."`
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
	NodeID     string `json:"nodeID,omitempty" jsonschema_description:"Unique ID of the node to find references for. Prefer this over SymbolName if available."`
	SymbolName string `json:"symbolName,omitempty" jsonschema_description:"The exact name of the symbol to find references for."`
	Path       string `json:"path,omitempty" jsonschema_description:"Optional relative file path to disambiguate SymbolName (e.g., 'pkg/db.go')."`
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
	NodeID     string `json:"nodeID,omitempty" jsonschema_description:"Unique ID of the function/method. Use this for maximum precision."`
	SymbolName string `json:"symbolName,omitempty" jsonschema_description:"Name of the function/method. Use if NodeID is unknown."`
	Path       string `json:"path,omitempty" jsonschema_description:"Optional relative path to disambiguate SymbolName."`
	Direction  string `json:"direction,omitempty" jsonschema_description:"Direction of calls: 'incoming', 'outgoing' (default), or 'both'."`
	Depth      int    `json:"depth,omitempty" jsonschema_description:"Max depth of the call graph (default 1). Higher depth is more expensive."`
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

type findImplementationsInput struct {
	NodeID string `json:"nodeId" jsonschema_description:"Unique ID of the interface or class to find implementations for."`
}

func (m *Manager) handleFindImplementations(boundProjectID string) sdkmcp.ToolHandlerFor[findImplementationsInput, models.FindImplementationsResponse] {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input findImplementationsInput) (*sdkmcp.CallToolResult, models.FindImplementationsResponse, error) {
		projectID, err := m.resolveProjectID(boundProjectID)
		if err != nil {
			return nil, models.FindImplementationsResponse{}, err
		}

		res, err := m.projectService.FindImplementations(projectID, input.NodeID)
		if err != nil {
			return nil, models.FindImplementationsResponse{}, err
		}

        var out models.FindImplementationsResponse
        if res != nil {
            out = *res
        }

		return nil, out, nil
	}
}

type getPackageGraphInput struct {
	Depth int `json:"depth,omitempty" jsonschema_description:"Max depth of the package tree (default 0, unlimited). Use for high-level overview."`
}

func (m *Manager) handleGetPackageGraph(boundProjectID string) sdkmcp.ToolHandlerFor[getPackageGraphInput, models.PackageGraphResponse] {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input getPackageGraphInput) (*sdkmcp.CallToolResult, models.PackageGraphResponse, error) {
		projectID, err := m.resolveProjectID(boundProjectID)
		if err != nil {
			return nil, nil, err
		}

		res, err := m.projectService.GetPackageGraph(projectID, input.Depth)
		if err != nil {
			return nil, nil, err
		}

		return nil, res, nil
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
	Path      string `json:"path,omitempty" jsonschema_description:"Optional relative sub-path to list files from."`
	Extension string `json:"extension,omitempty" jsonschema_description:"Filter results by extension (e.g., '.go', '.ts')."`
	Depth     *int   `json:"depth,omitempty" jsonschema_description:"Max recursion depth (default 1, 0 = unlimited). Clearer for bread-first browsing if set explicitly."`
}

type listFileOutput struct {
	Files [][]any `json:"files" jsonschema_description:"Table of files [Name, Lang, Size, Lines, Sym]"`
	Dirs  [][]any `json:"dirs" jsonschema_description:"Table of directories [Name, Items]"`
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
	Query string `json:"query" jsonschema_description:"Semantic natural language query (e.g., 'how is authentication handled?'). Clearer queries yield better results."`
	K     int    `json:"k,omitempty" jsonschema_description:"Max number of chunks to return (default 8, max 50)."`
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
	Query string `json:"query" jsonschema_description:"Concept or feature to search for (e.g., 'data persistence layer')."`
	K     int    `json:"k,omitempty" jsonschema_description:"Max number of files to return (default 5, max 20)."`
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
	Path  string `json:"path" jsonschema_description:"Relative file path from the project root."`
	Depth int    `json:"depth,omitempty" jsonschema_description:"Max recursion depth for symbols (1 = top-level symbols only)."`
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
	ID           []string `json:"id" jsonschema_description:"List of IDs. Supports FUZZY IDs like 'path|symbolName' or 'path|Lrange' (e.g. 'main.go|Init' or 'app.go|L10-20')."`
	CollapseBody bool     `json:"collapseBody,omitempty" jsonschema_description:"True to hide large function bodies, returning only signatures/headers."`
}

type grepSearchInput struct {
	Query   string `json:"query" jsonschema_description:"String or regular expression pattern."`
	IsRegex bool   `json:"isRegex,omitempty" jsonschema_description:"Interpret query as a regular expression (default false)."`
	Path    string `json:"path,omitempty" jsonschema_description:"Optional relative sub-path to restrict search."`
	Limit   int    `json:"limit,omitempty" jsonschema_description:"Max total matches to return (default 100, max 500)."`
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

type findTodosInput struct {
	Category string `json:"category,omitempty" jsonschema_description:"Optional category filter (e.g., 'FIXME', 'TODO') to limit results."`
}

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

		// Default depth to 1 if not specified, 0 means unlimited
		effectiveDepth := 1
		if input.Depth != nil {
			effectiveDepth = *input.Depth
		}
		
		previews, err := m.projectService.GetProjectStructure(projectID, input.Path, effectiveDepth)
		if err != nil {
			return nil, listFileOutput{}, err
		}

		files := [][]any{{"Name", "Lang", "Size", "Lines", "Sym"}}
		dirs := [][]any{{"Name", "Items"}}
		extension := strings.ToLower(input.Extension)
		
		for _, p := range previews {
			// Apply extension filter if specified
			if !p.IsDir && extension != "" {
				if !strings.HasSuffix(strings.ToLower(p.RelativePath), extension) {
					continue
				}
			}

			if p.IsDir {
				dirs = append(dirs, []any{p.RelativePath, p.ItemCount})
			} else {
				lang := strings.Join(p.Languages, ",")
				files = append(files, []any{
					p.RelativePath,
					lang,
					p.Size,
					p.Lines,
					p.Symbols,
				})
			}
		}

		return nil, listFileOutput{Files: files, Dirs: dirs}, nil
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
			chunks, err := m.projectService.GetChunksByIDFuzzy(projectID, id)
			if err != nil {
				return nil, nodeSourceOutput{}, fmt.Errorf("error fetching id %s: %w", id, err)
			}
	
			for _, chunk := range chunks {
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
					ID:        chunk.ID, // Include the real chunk ID
					FilePath:  chunk.FilePath,
					Source:    source,
					StartLine: chunk.LineStart,
					EndLine:   chunk.LineEnd,
					Language:  chunk.Language,
					Symbol:    chunk.SymbolName,
				})
			}
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

		// Apply category filter if requested
		if input.Category != "" {
			upperCat := strings.ToUpper(input.Category)
			filtered := make(map[string][]string)
			if ids, ok := resp.Categories[upperCat]; ok {
				filtered[upperCat] = ids
				
				// Update stats for the filtered view
				resp.Stats.Total = len(ids)
				resp.Stats.ByCategory = map[string]int{upperCat: len(ids)}
			} else {
				resp.Stats.Total = 0
				resp.Stats.ByCategory = make(map[string]int)
			}
			resp.Categories = filtered
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
