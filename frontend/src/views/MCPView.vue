<!--
  File: views/MCPView.vue
  Purpose: MCP (Model Context Protocol) server management interface.
  Author: CodeTextor project
  Notes: Allows configuration, start/stop, and monitoring of MCP server.
-->

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import { useCurrentProject } from '../composables/useCurrentProject';
import { mockBackend } from '../services/mockBackend';
import type { MCPServerConfig, MCPServerStatus, MCPTool } from '../types';

// Get current project
const { currentProject } = useCurrentProject();

// State
const config = ref<MCPServerConfig>({
  host: 'localhost',
  port: 3000,
  protocol: 'http',
  autoStart: false,
  maxConnections: 10
});

const status = ref<MCPServerStatus>({
  isRunning: false,
  uptime: 0,
  activeConnections: 0,
  totalRequests: 0,
  averageResponseTime: 0
});

const tools = ref<MCPTool[]>([]);
const isStarting = ref(false);
const isStopping = ref(false);
const isLoadingConfig = ref(false);

let statusInterval: number | null = null;

/**
 * Loads MCP server configuration.
 */
const loadConfig = async () => {
  isLoadingConfig.value = true;
  try {
    config.value = await mockBackend.getMCPConfig();
  } catch (error) {
    console.error('Failed to load MCP config:', error);
  } finally {
    isLoadingConfig.value = false;
  }
};

/**
 * Saves MCP server configuration.
 */
const saveConfig = async () => {
  try {
    await mockBackend.updateMCPConfig(config.value);
    alert('Configuration saved successfully');
  } catch (error) {
    console.error('Failed to save MCP config:', error);
    alert('Failed to save configuration');
  }
};

/**
 * Starts the MCP server.
 */
const startServer = async () => {
  isStarting.value = true;
  try {
    await mockBackend.startMCPServer();
    await updateStatus();
    startStatusPolling();
  } catch (error) {
    console.error('Failed to start MCP server:', error);
    alert('Failed to start server');
  } finally {
    isStarting.value = false;
  }
};

/**
 * Stops the MCP server.
 */
const stopServer = async () => {
  isStopping.value = true;
  try {
    await mockBackend.stopMCPServer();
    await updateStatus();
    stopStatusPolling();
  } catch (error) {
    console.error('Failed to stop MCP server:', error);
    alert('Failed to stop server');
  } finally {
    isStopping.value = false;
  }
};

/**
 * Updates server status.
 */
const updateStatus = async () => {
  try {
    status.value = await mockBackend.getMCPStatus();
  } catch (error) {
    console.error('Failed to update status:', error);
  }
};

/**
 * Loads available MCP tools.
 */
const loadTools = async () => {
  try {
    tools.value = await mockBackend.getMCPTools();
  } catch (error) {
    console.error('Failed to load tools:', error);
  }
};

/**
 * Toggles tool enabled state.
 * @param toolName - Name of tool to toggle
 */
const toggleTool = async (toolName: string) => {
  try {
    await mockBackend.toggleMCPTool(toolName);
    await loadTools();
  } catch (error) {
    console.error('Failed to toggle tool:', error);
  }
};

/**
 * Starts polling for status updates.
 */
const startStatusPolling = () => {
  if (statusInterval) return;
  statusInterval = window.setInterval(updateStatus, 2000);
};

/**
 * Stops polling for status updates.
 */
const stopStatusPolling = () => {
  if (statusInterval) {
    clearInterval(statusInterval);
    statusInterval = null;
  }
};

/**
 * Formats uptime seconds to readable string.
 * @param seconds - Uptime in seconds
 * @returns Formatted uptime string
 */
const formatUptime = (seconds: number): string => {
  if (seconds === 0) return 'Not running';
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = seconds % 60;

  if (hours > 0) {
    return `${hours}h ${minutes}m ${secs}s`;
  } else if (minutes > 0) {
    return `${minutes}m ${secs}s`;
  }
  return `${secs}s`;
};

/**
 * Copies server URL to clipboard.
 */
const copyServerURL = () => {
  const url = `${config.value.protocol}://${config.value.host}:${config.value.port}`;
  navigator.clipboard.writeText(url);
  alert('Server URL copied to clipboard!');
};

// Lifecycle hooks
onMounted(async () => {
  await loadConfig();
  await updateStatus();
  await loadTools();

  if (status.value.isRunning) {
    startStatusPolling();
  }
});

onUnmounted(() => {
  stopStatusPolling();
});
</script>

<template>
  <div class="mcp-view">
    <!-- Project Context Info -->
    <div v-if="currentProject" class="info-banner">
      <span class="info-icon">ℹ️</span>
      <span>MCP server will provide context for project: <strong>{{ currentProject.name }}</strong></span>
    </div>
    <div v-else class="warning-banner">
      <span class="warning-icon">⚠️</span>
      <span>No project selected. MCP server can run but tools won't have project context.</span>
    </div>

    <!-- Server Status -->
    <div class="section status-section">
      <div class="section-header">
        <h3>Server Status</h3>
        <div :class="['status-indicator', { active: status.isRunning }]">
          {{ status.isRunning ? '● Running' : '○ Stopped' }}
        </div>
      </div>

      <div class="status-grid">
        <div class="status-card">
          <div class="status-label">Uptime</div>
          <div class="status-value">{{ formatUptime(status.uptime) }}</div>
        </div>
        <div class="status-card">
          <div class="status-label">Active Connections</div>
          <div class="status-value">{{ status.activeConnections }}</div>
        </div>
        <div class="status-card">
          <div class="status-label">Total Requests</div>
          <div class="status-value">{{ status.totalRequests }}</div>
        </div>
        <div class="status-card">
          <div class="status-label">Avg Response Time</div>
          <div class="status-value">{{ status.averageResponseTime.toFixed(1) }}ms</div>
        </div>
      </div>

      <div class="server-controls">
        <button
          v-if="!status.isRunning"
          @click="startServer"
          :disabled="isStarting"
          class="btn btn-success"
        >
          {{ isStarting ? 'Starting...' : '▶ Start Server' }}
        </button>
        <button
          v-else
          @click="stopServer"
          :disabled="isStopping"
          class="btn btn-danger"
        >
          {{ isStopping ? 'Stopping...' : '■ Stop Server' }}
        </button>
        <button @click="copyServerURL" class="btn btn-secondary">
          📋 Copy URL
        </button>
      </div>
    </div>

    <!-- Configuration -->
    <div class="section config-section">
      <h3>Configuration</h3>

      <div class="config-grid">
        <div class="form-group">
          <label for="host">Host</label>
          <input
            id="host"
            v-model="config.host"
            type="text"
            class="input-text"
            :disabled="status.isRunning"
          />
        </div>

        <div class="form-group">
          <label for="port">Port</label>
          <input
            id="port"
            v-model.number="config.port"
            type="number"
            min="1024"
            max="65535"
            class="input-text"
            :disabled="status.isRunning"
          />
        </div>

        <div class="form-group">
          <label for="protocol">Protocol</label>
          <select
            id="protocol"
            v-model="config.protocol"
            class="input-select"
            :disabled="status.isRunning"
          >
            <option value="http">HTTP</option>
            <option value="stdio">STDIO</option>
          </select>
        </div>

        <div class="form-group">
          <label for="maxConnections">Max Connections</label>
          <input
            id="maxConnections"
            v-model.number="config.maxConnections"
            type="number"
            min="1"
            max="100"
            class="input-text"
            :disabled="status.isRunning"
          />
        </div>
      </div>

      <div class="form-group checkbox-group">
        <label>
          <input
            v-model="config.autoStart"
            type="checkbox"
            :disabled="status.isRunning"
          />
          <span>Auto-start server on application launch</span>
        </label>
      </div>

      <div class="form-actions">
        <button
          @click="saveConfig"
          :disabled="status.isRunning || isLoadingConfig"
          class="btn btn-primary"
        >
          💾 Save Configuration
        </button>
      </div>

      <div v-if="status.isRunning" class="warning-message">
        ⚠️ Stop the server to modify configuration
      </div>
    </div>

    <!-- Available Tools -->
    <div class="section tools-section">
      <h3>Available Tools</h3>
      <p class="section-description">
        MCP tools exposed to IDE clients for code understanding and navigation
      </p>

      <div class="tools-list">
        <div
          v-for="tool in tools"
          :key="tool.name"
          :class="['tool-item', { disabled: !tool.enabled }]"
        >
          <div class="tool-header">
            <div class="tool-name">{{ tool.name }}</div>
            <div class="tool-badge">{{ tool.callCount }} calls</div>
          </div>
          <div class="tool-description">{{ tool.description }}</div>
          <div class="tool-actions">
            <button
              @click="toggleTool(tool.name)"
              :class="['btn-toggle', { enabled: tool.enabled }]"
            >
              {{ tool.enabled ? 'Enabled' : 'Disabled' }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Connection Info -->
    <div class="section info-section">
      <h3>Connection Information</h3>
      <div class="info-box">
        <p><strong>Server URL:</strong></p>
        <code class="server-url">{{ config.protocol }}://{{ config.host }}:{{ config.port }}</code>

        <p><strong>Claude Desktop Configuration:</strong></p>
        <pre class="config-snippet"><code>{
  "mcpServers": {
    "codetextor": {
      "command": "{{ config.protocol }}://{{ config.host }}:{{ config.port }}"
    }
  }
}</code></pre>
      </div>
    </div>
  </div>
</template>

<style scoped>
.mcp-view {
  max-width: 1200px;
  margin: 0 auto;
}

.section {
  background: #252526;
  border: 1px solid #3e3e42;
  border-radius: 8px;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
}

.section h3 {
  margin: 0 0 1rem 0;
  color: #d4d4d4;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.5rem;
}

.section-header h3 {
  margin: 0;
}

.status-indicator {
  padding: 0.5rem 1rem;
  border-radius: 20px;
  font-size: 0.9rem;
  font-weight: 600;
  background: #6c757d;
  color: white;
}

.status-indicator.active {
  background: #28a745;
  animation: pulse 2s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.7; }
}

.status-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
  margin-bottom: 1.5rem;
}

.status-card {
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 6px;
  padding: 1rem;
}

.status-label {
  color: #858585;
  font-size: 0.85rem;
  margin-bottom: 0.5rem;
}

.status-value {
  color: #d4d4d4;
  font-size: 1.5rem;
  font-weight: 600;
}

.server-controls {
  display: flex;
  gap: 0.75rem;
}

.btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 6px;
  font-size: 0.95rem;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-success {
  background: #28a745;
  color: white;
}

.btn-success:hover:not(:disabled) {
  background: #218838;
}

.btn-danger {
  background: #dc3545;
  color: white;
}

.btn-danger:hover:not(:disabled) {
  background: #c82333;
}

.btn-primary {
  background: #007acc;
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background: #005a9e;
}

.btn-secondary {
  background: #6c757d;
  color: white;
}

.btn-secondary:hover {
  background: #5a6268;
}

.config-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 1rem;
  margin-bottom: 1rem;
}

.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  color: #d4d4d4;
  font-weight: 500;
}

.input-text, .input-select {
  width: 100%;
  padding: 0.75rem;
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 4px;
  color: #d4d4d4;
  font-size: 0.95rem;
}

.input-text:disabled, .input-select:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.checkbox-group label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
}

.checkbox-group input[type="checkbox"] {
  width: 18px;
  height: 18px;
}

.form-actions {
  display: flex;
  gap: 0.75rem;
}

.warning-message {
  margin-top: 1rem;
  padding: 0.75rem;
  background: rgba(255, 193, 7, 0.1);
  border: 1px solid #ffc107;
  border-radius: 4px;
  color: #ffc107;
  font-size: 0.9rem;
}

.section-description {
  margin: -0.5rem 0 1rem 0;
  color: #858585;
  font-size: 0.9rem;
}

.tools-list {
  display: grid;
  gap: 1rem;
}

.tool-item {
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 6px;
  padding: 1rem;
  transition: all 0.2s ease;
}

.tool-item.disabled {
  opacity: 0.6;
}

.tool-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5rem;
}

.tool-name {
  font-weight: 600;
  color: #d4d4d4;
  font-family: 'Courier New', monospace;
}

.tool-badge {
  padding: 0.25rem 0.75rem;
  background: #007acc;
  border-radius: 12px;
  font-size: 0.75rem;
  color: white;
}

.tool-description {
  color: #858585;
  font-size: 0.9rem;
  margin-bottom: 0.75rem;
}

.tool-actions {
  display: flex;
  gap: 0.5rem;
}

.btn-toggle {
  padding: 0.4rem 1rem;
  border: 1px solid #3e3e42;
  border-radius: 4px;
  font-size: 0.85rem;
  cursor: pointer;
  transition: all 0.2s ease;
  background: #6c757d;
  color: white;
}

.btn-toggle.enabled {
  background: #28a745;
  border-color: #28a745;
}

.btn-toggle:hover {
  opacity: 0.8;
}

.info-box {
  background: #1e1e1e;
  border: 1px solid #3e3e42;
  border-radius: 6px;
  padding: 1.5rem;
}

.info-box p {
  margin: 1rem 0 0.5rem 0;
  color: #d4d4d4;
}

.info-box p:first-child {
  margin-top: 0;
}

.server-url {
  display: block;
  padding: 0.75rem;
  background: #0d1117;
  border: 1px solid #3e3e42;
  border-radius: 4px;
  color: #58a6ff;
  font-family: 'Courier New', monospace;
  font-size: 0.95rem;
}

.config-snippet {
  margin: 0;
  padding: 1rem;
  background: #0d1117;
  border: 1px solid #3e3e42;
  border-radius: 4px;
  overflow-x: auto;
}

.config-snippet code {
  color: #d4d4d4;
  font-family: 'Courier New', monospace;
  font-size: 0.9rem;
  line-height: 1.5;
}

/* Project context banners */
.info-banner,
.warning-banner {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 1rem;
  border-radius: 6px;
  margin-bottom: 1.5rem;
  font-size: 0.95rem;
}

.info-banner {
  background: #1a3a5a;
  border: 1px solid #007acc;
  color: #7fc7ff;
}

.warning-banner {
  background: #5a4a1a;
  border: 1px solid #ffc107;
  color: #ffd966;
}

.info-icon,
.warning-icon {
  font-size: 1.5rem;
}
</style>
