<template>
  <div class="space-y-4">
    <div class="flex items-center gap-4">
      <div class="flex gap-2">
        <button
          @click="toggleConnection"
          :class="[
            'font-semibold py-2 px-4 rounded transition-colors text-white',
            wsConnected ? 'bg-red-600 hover:bg-red-700' : 'bg-green-600 hover:bg-green-700'
          ]"
        >
          {{ wsConnected ? 'Disconnect' : 'Connect' }}
        </button>
        <button
          @click="clearLogs"
          class="bg-slate-600 hover:bg-slate-500 text-white font-semibold py-2 px-4 rounded transition-colors"
        >
          Clear
        </button>
      </div>

      <!-- Connection Status -->
      <div class="flex items-center gap-2">
        <div :class="['w-2 h-2 rounded-full', wsConnected ? 'bg-green-400 animate-pulse' : 'bg-red-400']"></div>
        <span class="text-slate-400 text-sm">
          {{ wsConnected ? 'Live' : reconnecting ? `Reconnecting (${reconnectAttempt})...` : 'Disconnected' }}
        </span>
      </div>

      <!-- Filter -->
      <div class="flex-1 flex gap-2">
        <select
          v-model="levelFilter"
          class="bg-slate-700 text-slate-300 border border-slate-600 rounded px-3 py-2 text-sm"
        >
          <option value="all">All Levels</option>
          <option value="info">Info</option>
          <option value="warn">Warning</option>
          <option value="error">Error</option>
        </select>
        <input
          v-model="searchFilter"
          placeholder="Search logs..."
          class="flex-1 bg-slate-700 text-slate-300 border border-slate-600 rounded px-3 py-2 text-sm"
        />
      </div>
    </div>

    <!-- Log Display -->
    <div
      ref="logContainer"
      class="bg-slate-950 border border-slate-600 rounded p-4 h-96 overflow-y-auto font-mono text-sm"
    >
      <div v-if="filteredLogs.length === 0" class="text-slate-500">
        {{ logs.length === 0 ? 'No logs yet. Waiting for messages...' : 'No logs match the current filter.' }}
      </div>
      <div v-else class="space-y-0.5">
        <div
          v-for="(log, index) in filteredLogs"
          :key="index"
          :class="['py-0.5 px-1 rounded', logLineClass(log.level)]"
        >
          <span class="text-slate-600 mr-2">{{ formatTime(log.timestamp) }}</span>
          <span :class="levelBadgeClass(log.level)" class="text-xs font-bold mr-2 uppercase">{{ log.level }}</span>
          <span v-if="log.source" class="text-slate-500 mr-2">[{{ log.source }}]</span>
          <span class="text-slate-300">{{ log.message }}</span>
        </div>
      </div>
    </div>

    <!-- Log Count -->
    <div class="text-slate-500 text-xs">
      {{ filteredLogs.length }} of {{ logs.length }} messages
      <span v-if="logs.length >= maxHistory"> (history capped at {{ maxHistory }})</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue';

interface LogEntry {
  timestamp: number;
  level: string;
  message: string;
  source: string;
}

const maxHistory = 2000;
const logs = ref<LogEntry[]>([]);
const wsConnected = ref(false);
const reconnecting = ref(false);
const reconnectAttempt = ref(0);
const levelFilter = ref('all');
const searchFilter = ref('');
const logContainer = ref<HTMLElement | null>(null);

let ws: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let autoScroll = true;
// Guards against the onclose→scheduleReconnect race when the user clicks Disconnect.
// Without this, ws.close() triggers onclose asynchronously AFTER disconnectWebSocket()
// has already cleared reconnectTimer, so scheduleReconnect() sees null and re-arms.
let intentionalDisconnect = false;
// Generation counter to invalidate stale onclose/onerror callbacks. Each call to
// connectWebSocket() increments this; callbacks that captured an older generation
// are silently ignored, preventing duplicate connections after Disconnect→Connect.
let wsGeneration = 0;

const filteredLogs = computed(() => {
  return logs.value.filter((log) => {
    if (levelFilter.value !== 'all' && log.level !== levelFilter.value) return false;
    if (searchFilter.value && !log.message.toLowerCase().includes(searchFilter.value.toLowerCase())) return false;
    return true;
  });
});

onMounted(() => {
  connectWebSocket();
});

onUnmounted(() => {
  disconnectWebSocket();
  if (reconnectTimer) clearTimeout(reconnectTimer);
});

watch(filteredLogs, () => {
  if (autoScroll) {
    nextTick(() => scrollToBottom());
  }
});

function connectWebSocket() {
  // Bump generation so stale onclose/onerror callbacks from a previous
  // socket become no-ops. This closes the Disconnect→Connect race where
  // the old socket's queued onclose fires after intentionalDisconnect
  // has already been reset, which would otherwise trigger a duplicate
  // connection via scheduleReconnect().
  const generation = ++wsGeneration;
  intentionalDisconnect = false;
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = window.location.host;
  const wsUrl = `${protocol}//${host}/api/ws/logs`;

  try {
    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      if (generation !== wsGeneration) return;
      wsConnected.value = true;
      reconnecting.value = false;
      reconnectAttempt.value = 0;
    };

    ws.onmessage = (event) => {
      if (generation !== wsGeneration) return;
      // Messages can be newline-separated (batched writes)
      const lines = event.data.split('\n');
      for (const line of lines) {
        if (!line.trim()) continue;
        try {
          const entry: LogEntry = JSON.parse(line);
          logs.value.push(entry);
        } catch {
          // If not JSON, treat as plain text
          logs.value.push({
            timestamp: Math.floor(Date.now() / 1000),
            level: 'info',
            message: line,
            source: 'raw',
          });
        }
      }
      // Cap history once after processing all lines in the batch, rather than
      // on every individual push, to avoid repeated array copies.
      if (logs.value.length > maxHistory) {
        logs.value = logs.value.slice(-maxHistory);
      }
    };

    ws.onclose = () => {
      if (generation !== wsGeneration) return;
      wsConnected.value = false;
      if (!intentionalDisconnect) {
        scheduleReconnect();
      }
    };

    ws.onerror = () => {
      if (generation !== wsGeneration) return;
      wsConnected.value = false;
    };
  } catch {
    wsConnected.value = false;
    scheduleReconnect();
  }
}

function disconnectWebSocket() {
  intentionalDisconnect = true;
  if (ws) {
    ws.close();
    ws = null;
  }
  wsConnected.value = false;
  reconnecting.value = false;
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
}

function scheduleReconnect() {
  if (reconnectTimer) return;
  reconnecting.value = true;
  reconnectAttempt.value++;

  // Exponential backoff: 1s, 2s, 4s, 8s, max 30s
  const delay = Math.min(1000 * Math.pow(2, reconnectAttempt.value - 1), 30000);

  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    connectWebSocket();
  }, delay);
}

function toggleConnection() {
  if (wsConnected.value) {
    disconnectWebSocket();
  } else {
    // Clear any pending auto-reconnect timer to prevent a duplicate
    // connection when the timer fires after this manual connect.
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
    reconnecting.value = false;
    reconnectAttempt.value = 0;
    connectWebSocket();
  }
}

function clearLogs() {
  logs.value = [];
}

function scrollToBottom() {
  if (logContainer.value) {
    logContainer.value.scrollTop = logContainer.value.scrollHeight;
  }
}

function formatTime(timestamp: number): string {
  if (!timestamp) return '';
  const d = new Date(timestamp * 1000);
  return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

function logLineClass(level: string): string {
  if (level === 'error') return 'bg-red-950/30';
  if (level === 'warn') return 'bg-yellow-950/30';
  return '';
}

function levelBadgeClass(level: string): string {
  if (level === 'error') return 'text-red-400';
  if (level === 'warn') return 'text-yellow-400';
  if (level === 'info') return 'text-blue-400';
  return 'text-slate-400';
}
</script>
