<!--
  File: views/MCPView.vue
  Purpose: MCP (Model Context Protocol) server management interface.
  Author: CodeTextor project
  Notes: Allows configuration, start/stop, and monitoring of MCP server.
-->

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { backend, models } from '../api/backend';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import type { MCPServerConfig, MCPServerStatus, MCPTool } from '../types';
import { useCurrentProject } from '../composables/useCurrentProject';

const { currentProject } = useCurrentProject();

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
const isLoadingConfig = ref(false);
const isStatusLoading = ref(false);
const isToolsLoading = ref(false);
const isTogglingServer = ref(false);
const notification = ref<{ type: 'success' | 'error'; message: string } | null>(null);

const serverUrl = computed(() => `${config.value.protocol}://${config.value.host}:${config.value.port}`);
const projectId = computed(() => currentProject.value?.id ?? '<project-id>');
const projectServerUrl = computed(() => `${serverUrl.value}/mcp/${projectId.value}`);
const currentProjectLabel = computed(() =>
  currentProject.value ? `${currentProject.value.name} (${currentProject.value.id})` : 'Nessun progetto selezionato'
);

let notificationTimer: number | null = null;
let statusUnsubscribe: (() => void) | null = null;
let toolsUnsubscribe: (() => void) | null = null;

const showNotification = (type: 'success' | 'error', message: string) => {
  if (notificationTimer) {
    clearTimeout(notificationTimer);
    notificationTimer = null;
  }
  notification.value = { type, message };
  notificationTimer = window.setTimeout(() => {
    notification.value = null;
    notificationTimer = null;
  }, 3000);
};

const dismissNotification = () => {
  if (notificationTimer) {
    clearTimeout(notificationTimer);
    notificationTimer = null;
  }
  notification.value = null;
};

const handleError = (context: string, error: unknown) => {
  console.error(context, error);
  const message =
    error instanceof Error ? error.message : typeof error === 'string' ? error : 'Unknown error';
  showNotification('error', `${context}: ${message}`);
};

const normalizeConfig = (cfg: models.MCPServerConfig): MCPServerConfig => ({
  host: cfg.host,
  port: cfg.port,
  protocol: cfg.protocol === 'stdio' ? 'stdio' : 'http',
  autoStart: cfg.autoStart,
  maxConnections: cfg.maxConnections
});

const applyConfig = (newConfig: MCPServerConfig) => {
  config.value = {
    host: newConfig.host,
    port: newConfig.port,
    protocol: newConfig.protocol,
    autoStart: newConfig.autoStart,
    maxConnections: newConfig.maxConnections
  };
};

const loadConfig = async () => {
  isLoadingConfig.value = true;
  try {
    const cfg = await backend.getMCPConfig();
    applyConfig(normalizeConfig(cfg));
  } catch (error) {
    handleError('Failed to load MCP config', error);
  } finally {
    isLoadingConfig.value = false;
  }
};

const startServer = async () => {
  try {
    await backend.startMCPServer();
    await updateStatus();
    showNotification('success', 'Server started');
  } catch (error) {
    handleError('Failed to start server', error);
  }
};

const stopServer = async () => {
  try {
    await backend.stopMCPServer();
    await updateStatus();
    showNotification('success', 'Server stopped');
  } catch (error) {
    handleError('Failed to stop server', error);
  }
};

const toggleServer = async () => {
  if (isTogglingServer.value) return;
  isTogglingServer.value = true;
  try {
    if (status.value.isRunning) {
      await stopServer();
    } else {
      await startServer();
    }
  } finally {
    isTogglingServer.value = false;
  }
};

const updateStatus = async () => {
  isStatusLoading.value = true;
  try {
    const next = await backend.getMCPStatus();
    status.value = {
      ...status.value,
      ...next,
      lastError: next.lastError
    };
  } catch (error) {
    handleError('Failed to refresh status', error);
  } finally {
    isStatusLoading.value = false;
  }
};

const loadTools = async () => {
  isToolsLoading.value = true;
  try {
    tools.value = await backend.getMCPTools();
  } catch (error) {
    handleError('Failed to load tools', error);
  } finally {
    isToolsLoading.value = false;
  }
};

const formatUptime = (seconds: number): string => {
  if (seconds === 0) return 'Not running';
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = seconds % 60;

  if (hours > 0) {
    return `${hours}h ${minutes}m ${secs}s`;
  }
  if (minutes > 0) {
    return `${minutes}m ${secs}s`;
  }
  return `${secs}s`;
};

onMounted(async () => {
  await loadConfig();
  await Promise.all([updateStatus(), loadTools()]);

  statusUnsubscribe = EventsOn('mcp:status', (payload: MCPServerStatus) => {
    status.value = {
      ...status.value,
      ...payload,
      lastError: payload.lastError
    };
  });

  toolsUnsubscribe = EventsOn('mcp:tools', (payload: MCPTool[]) => {
    tools.value = payload ?? [];
  });
});

onUnmounted(() => {
  dismissNotification();
  statusUnsubscribe?.();
  toolsUnsubscribe?.();
});
</script>

<template>
  <div class="mcp-view">
    <div class="notification-stack">
      <transition name="fade">
        <div v-if="notification" :class="['alert-banner', notification.type]">
          <span>{{ notification.message }}</span>
          <button
            type="button"
            class="alert-close"
            aria-label="Dismiss notification"
            @click="dismissNotification"
          >
            ×
          </button>
        </div>
      </transition>
    </div>

    <!-- Server Status -->
    <div class="section status-section">
      <div class="section-header">
        <h3>Server Status</h3>
        <div class="section-header-meta">
          <span v-if="isStatusLoading" class="status-refresh">Refreshing...</span>
          <label class="toggle">
            <input
              type="checkbox"
              :checked="status.isRunning"
              @change="toggleServer"
              :disabled="isTogglingServer"
            />
            <span class="slider"></span>
            <span class="toggle-label">{{ status.isRunning ? 'On' : 'Off' }}</span>
          </label>
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

      <div v-if="status.lastError" class="status-error">
        ⚠️ {{ status.lastError }}
      </div>

      <div class="server-controls">
      </div>
    </div>

    <!-- Available Tools -->
    <div class="section tools-section">
      <h3>Available Tools</h3>
      <p class="section-description">
        Tools are always enabled; IDE clients can call them for context and navigation.
      </p>

      <div v-if="isToolsLoading" class="loading-indicator">Loading tools...</div>
      <div v-else-if="!tools.length" class="empty-state">
        No tools are registered yet.
      </div>
      <div v-else class="tools-list">
        <div
          v-for="tool in tools"
          :key="tool.name"
          class="tool-item"
        >
          <div class="tool-name">{{ tool.name }}</div>
          <div class="tool-description">{{ tool.description }}</div>
        </div>
      </div>
    </div>

    <!-- Connection Info -->
    <div class="section info-section">
      <h3>Connection Information</h3>
      <div class="info-box">
        <p class="info-label">
          Server URL per progetto
          <span class="info-sub" :class="{ muted: !currentProject }">
            {{ currentProjectLabel }}
          </span>
        </p>
        <code class="server-url">{{ projectServerUrl }}</code>

        <div class="snippet-grid">
          <div class="snippet-card">
            <div class="snippet-title">Codex CLI (`~/.codex/config.toml`)</div>
            <pre class="config-snippet"><code>[mcp_servers.codetextor]
url = "{{ projectServerUrl }}"
transport = "http"
enabled = true

[features]
rmcp_client = true</code></pre>
          </div>
          <div class="snippet-card">
            <div class="snippet-title">Claude Code CLI</div>
            <pre class="config-snippet"><code>
claude mcp add --transport http codetextor {{ projectServerUrl }}
</code></pre>
          </div>
          <div class="snippet-card">
            <div class="snippet-title">Claude Code (`.mcp.json`)</div>
            <pre class="config-snippet"><code>{
  "mcpServers": {
    "codetextor": {
      "type": "http",
      "url": "{{ projectServerUrl }}"
    }
  }
}</code></pre>
          </div>
          <div class="snippet-card">
            <div class="snippet-title">VS Code / Cursor / Windsurf </div>
            <pre class="config-snippet"><code>{
  "mcpServers": {
    "codetextor": {
      "type": "http",
      "url": "{{ projectServerUrl }}"
    }
  }
}</code></pre>
          </div>
        </div>
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

.btn-secondary:hover:not(:disabled) {
  background: #5a6268;
}

.toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}

.toggle input {
  display: none;
}

.slider {
  position: relative;
  width: 48px;
  height: 26px;
  background: #3e3e42;
  border-radius: 26px;
  transition: background 0.2s ease;
}

.slider::after {
  content: '';
  position: absolute;
  top: 3px;
  left: 3px;
  width: 20px;
  height: 20px;
  background: #d4d4d4;
  border-radius: 50%;
  transition: transform 0.2s ease;
}

.toggle input:checked + .slider {
  background: #28a745;
}

.toggle input:checked + .slider::after {
  transform: translateX(22px);
}

.toggle-label {
  color: #d4d4d4;
  font-weight: 600;
}

.notification-stack {
  position: fixed;
  bottom: 72px;
  right: 12px;
  width: min(360px, calc(100vw - 24px));
  z-index: 2000;
  pointer-events: none;
}

.notification-stack .alert-banner {
  pointer-events: auto;
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
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
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

.tool-description {
  color: #858585;
  font-size: 0.9rem;
  margin-bottom: 0.75rem;
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

.info-sub {
  display: inline-block;
  margin-left: 0.35rem;
  color: #7ec9ff;
  font-weight: 500;
}

.info-sub.muted {
  color: #858585;
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
  color: #7ec9ff;
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
  text-align: left;
}

.config-snippet code {
  color: #d4d4d4;
  font-family: 'Courier New', monospace;
  font-size: 0.9rem;
  line-height: 1.5;
  text-align: left;
}

.info-label {
  color: #d4d4d4;
  font-weight: 600;
  margin: 0 0 0.5rem 0;
}

.snippet-grid {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  margin-top: 1rem;
}

.snippet-card {
  background: #0d1117;
  border: 1px solid #3e3e42;
  border-radius: 6px;
  padding: 0.75rem;
}

.snippet-title {
  margin: 0 0 0.5rem 0;
  color: #e5e5e5;
  font-weight: 600;
}

.snippet-hint {
  margin: 0.5rem 0 0 0;
  color: #858585;
  font-size: 0.9rem;
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

.banner-content {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.banner-title {
  margin: 0;
  color: #e5e5e5;
  font-weight: 600;
}

.banner-text {
  margin: 0;
  color: #d4d4d4;
  font-size: 0.95rem;
}

.alert-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.85rem 1rem;
  border-radius: 6px;
  margin-bottom: 1.5rem;
  border: 1px solid transparent;
  font-size: 0.95rem;
}

.alert-banner.success {
  background: rgba(40, 167, 69, 0.15);
  border-color: #28a745;
  color: #d4ffd4;
}

.alert-banner.error {
  background: rgba(220, 53, 69, 0.15);
  border-color: #dc3545;
  color: #ffccd5;
}

.alert-close {
  background: transparent;
  border: none;
  color: inherit;
  font-size: 1.2rem;
  cursor: pointer;
}

.section-header-meta {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.status-refresh {
  font-size: 0.85rem;
  color: #9cdcfe;
}

.status-error {
  margin-bottom: 1rem;
  padding: 0.75rem 1rem;
  border-radius: 4px;
  background: rgba(220, 53, 69, 0.1);
  border: 1px solid rgba(220, 53, 69, 0.4);
  color: #f28b9c;
  font-size: 0.9rem;
}

.helper-text {
  margin-top: 0.35rem;
  font-size: 0.8rem;
  color: #858585;
}

.loading-indicator,
.empty-state {
  padding: 1rem;
  border: 1px dashed #3e3e42;
  border-radius: 6px;
  text-align: center;
  color: #d4d4d4;
}

.loading-indicator {
  color: #9cdcfe;
}

.empty-state {
  color: #bbbbbb;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
