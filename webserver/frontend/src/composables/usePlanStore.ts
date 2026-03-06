import { ref, computed, watch } from 'vue';
import type { ComputedRef } from 'vue';
import axios from 'axios';

export interface PlanItemSnapshot {
  item_id: string;
  item_type: string; // "track" | "album" | "artist" | "playlist" | "m3u"
  name: string;
  status: string; // "pending" | "in_progress" | "completed" | "failed" | "skipped"
  error?: string;
  raw_output?: string;
  file_path?: string;
  parent_id?: string;
  child_ids?: string[];
  spotify_url?: string;
  youtube_url?: string;
  metadata?: Record<string, unknown>;
  progress: number;
  started_at?: number;
  completed_at?: number;
}

export interface PlanStats {
  total: number;
  completed: number;
  failed: number;
  pending: number;
  in_progress: number;
  skipped: number;
}

export interface PlanSnapshot {
  phase: string;
  items: PlanItemSnapshot[];
  generated_at?: string;
  config_hash?: string;
  stats: PlanStats;
}

interface PlanMessage {
  type: string;
  timestamp: number;
  item_id?: string;
  status?: string;
  error?: string;
  raw_output?: string;
  file_path?: string;
  name?: string;
  phase?: string;
  message?: string;
  items_found?: number;
  plan?: PlanSnapshot;
}

const CONFIG_PATH = '/download/config.yaml';
const ETA_WINDOW_MS = 60_000;
const COMPLETION_HISTORY_MAX = 100;

// Singleton state shared across all usePlanStore() callers
let ws: WebSocket | null = null;
let wsGeneration = 0;
let intentionalDisconnect = false;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let reconnectAttempt = 0;
let fallbackPollTimer: ReturnType<typeof setTimeout> | null = null;
let elapsedInterval: ReturnType<typeof setInterval> | null = null;

const items = ref<Map<string, PlanItemSnapshot>>(new Map());
const phase = ref<'idle' | 'generating' | 'downloading' | 'complete' | 'ready'>('idle');
const wsConnected = ref(false);
const generationProgress = ref('');
const generationItemsFound = ref(0);
const completionTimes: number[] = [];
const startTime = ref<number | null>(null);
const elapsedSeconds = ref(0);

function normalizePhase(p: string): 'idle' | 'generating' | 'downloading' | 'complete' | 'ready' {
  if (p === 'idle' || p === 'generating' || p === 'downloading' || p === 'complete' || p === 'ready') {
    return p;
  }
  return 'idle';
}

function applyPlanSnapshot(snap: PlanSnapshot) {
  const newMap = new Map<string, PlanItemSnapshot>();
  for (const item of snap.items) {
    newMap.set(item.item_id, { ...item });
  }
  items.value = newMap;
  phase.value = normalizePhase(snap.phase);
}

function applyItemUpdate(msg: PlanMessage) {
  if (!msg.item_id) return;
  const existing = items.value.get(msg.item_id);
  if (!existing) return;
  const updated: PlanItemSnapshot = { ...existing };
  if (msg.status !== undefined) updated.status = msg.status;
  if (msg.error !== undefined) updated.error = msg.error;
  if (msg.raw_output !== undefined) updated.raw_output = msg.raw_output;
  if (msg.file_path !== undefined) updated.file_path = msg.file_path;
  if (msg.name !== undefined) updated.name = msg.name;
  if (msg.status === 'completed') {
    completionTimes.push(Date.now());
    if (completionTimes.length > COMPLETION_HISTORY_MAX) {
      completionTimes.shift();
    }
  }
  const nextMap = new Map(items.value);
  nextMap.set(msg.item_id, updated);
  items.value = nextMap;
}

const stats: ComputedRef<PlanStats> = computed(() => {
  const s: PlanStats = {
    total: 0,
    completed: 0,
    failed: 0,
    pending: 0,
    in_progress: 0,
    skipped: 0,
  };
  for (const item of items.value.values()) {
    if (item.item_type !== 'track') continue;
    s.total++;
    switch (item.status) {
      case 'completed':
        s.completed++;
        break;
      case 'failed':
        s.failed++;
        break;
      case 'pending':
        s.pending++;
        break;
      case 'in_progress':
        s.in_progress++;
        break;
      case 'skipped':
        s.skipped++;
        break;
    }
  }
  return s;
});

const activeDownloads: ComputedRef<PlanItemSnapshot[]> = computed(() => {
  return [...items.value.values()].filter(
    (i) => i.item_type === 'track' && i.status === 'in_progress'
  );
});

const failedItems: ComputedRef<PlanItemSnapshot[]> = computed(() => {
  return [...items.value.values()].filter(
    (i) => i.item_type === 'track' && i.status === 'failed'
  );
});

const containerTypes = new Set(['playlist', 'album', 'artist', 'm3u']);

const rootNodes: ComputedRef<PlanItemSnapshot[]> = computed(() => {
  const roots = [...items.value.values()].filter((i) => !i.parent_id);
  roots.sort((a, b) => {
    const aIsContainer = containerTypes.has(a.item_type);
    const bIsContainer = containerTypes.has(b.item_type);
    if (aIsContainer && !bIsContainer) return -1;
    if (!aIsContainer && bIsContainer) return 1;
    return 0;
  });
  return roots;
});

const speed: ComputedRef<number> = computed(() => {
  const now = Date.now();
  const cutoff = now - ETA_WINDOW_MS;
  const recent = completionTimes.filter((t) => t >= cutoff);
  return (recent.length / ETA_WINDOW_MS) * 60_000; // tracks per minute
});

const eta: ComputedRef<string | null> = computed(() => {
  const spd = speed.value;
  if (spd <= 0) return null;
  const s = stats.value;
  const remaining = s.pending + s.in_progress;
  if (remaining <= 0) return null;
  const minutes = remaining / spd;
  if (minutes < 60) {
    return `~${Math.ceil(minutes)}m`;
  }
  const h = Math.floor(minutes / 60);
  const m = Math.ceil(minutes % 60);
  return `~${h}h ${m}m`;
});

const progressPercent: ComputedRef<number> = computed(() => {
  const s = stats.value;
  if (s.total === 0) return 0;
  const done = s.completed + s.failed + s.skipped;
  return (done / s.total) * 100;
});

function getChildren(parentId: string): PlanItemSnapshot[] {
  return [...items.value.values()].filter((i) => i.parent_id === parentId);
}

function connectWebSocket() {
  const generation = ++wsGeneration;
  intentionalDisconnect = false;
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = window.location.host;
  const wsUrl = `${protocol}//${host}/api/ws/plan`;

  try {
    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      if (generation !== wsGeneration) return;
      wsConnected.value = true;
      reconnectAttempt = 0;
    };

    ws.onmessage = (event) => {
      if (generation !== wsGeneration) return;
      try {
        const msg: PlanMessage = JSON.parse(event.data);
        switch (msg.type) {
          case 'plan_loaded':
            if (msg.plan) applyPlanSnapshot(msg.plan);
            break;
          case 'phase_change':
            if (msg.phase) phase.value = normalizePhase(msg.phase);
            break;
          case 'item_update':
            applyItemUpdate(msg);
            break;
          case 'plan_progress':
            if (msg.message !== undefined) generationProgress.value = msg.message;
            if (msg.items_found !== undefined) generationItemsFound.value = msg.items_found;
            break;
        }
      } catch {
        // Ignore parse errors
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

function scheduleReconnect() {
  if (reconnectTimer) return;
  reconnectAttempt++;
  const delay = Math.min(1000 * Math.pow(2, reconnectAttempt - 1), 30000);
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    connectWebSocket();
  }, delay);
}

function disconnectWebSocket() {
  intentionalDisconnect = true;
  if (ws) {
    ws.close();
    ws = null;
  }
  wsConnected.value = false;
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
}

async function fetchPlanSnapshot() {
  try {
    const response = await axios.get<PlanSnapshot | null>('/api/plan');
    const snap = response.data;
    if (snap) applyPlanSnapshot(snap);
  } catch {
    // Ignore fetch errors
  }
}

function startElapsedTimer() {
  if (elapsedInterval) return;
  elapsedInterval = setInterval(() => {
    if (startTime.value === null) return;
    elapsedSeconds.value = (Date.now() - startTime.value) / 1000;
  }, 1000);
}

function stopElapsedTimer() {
  if (elapsedInterval) {
    clearInterval(elapsedInterval);
    elapsedInterval = null;
  }
}

watch(phase, (p) => {
  if (p === 'downloading') {
    startTime.value = Date.now();
    startElapsedTimer();
  } else {
    stopElapsedTimer();
  }
});

watch(wsConnected, (connected) => {
  if (connected) {
    if (fallbackPollTimer) {
      clearTimeout(fallbackPollTimer);
      fallbackPollTimer = null;
    }
  } else {
    if (!fallbackPollTimer) {
      const poll = () => {
        fallbackPollTimer = null;
        fetchPlanSnapshot();
        if (!wsConnected.value) {
          fallbackPollTimer = setTimeout(poll, 5000);
        }
      };
      fallbackPollTimer = setTimeout(poll, 5000);
    }
  }
});

async function startDownload(resume: boolean): Promise<void> {
  await axios.post('/api/download/run', {
    configPath: CONFIG_PATH,
    resume: resume ? 'true' : 'false',
  });
}

async function stopOperation(): Promise<void> {
  await axios.post('/api/download/stop');
}

/**
 * Plan store composable: manages plan state for the download dashboard.
 * Connects to WebSocket at /api/ws/plan for real-time updates and falls back
 * to GET /api/plan for snapshots when disconnected.
 */
export function usePlanStore() {
  return {
    phase,
    items,
    stats,
    wsConnected,
    generationProgress,
    generationItemsFound,
    activeDownloads,
    failedItems,
    rootNodes,
    eta,
    speed,
    elapsedSeconds,
    progressPercent,
    connect: () => {
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      reconnectAttempt = 0;
      connectWebSocket();
      fetchPlanSnapshot();
    },
    disconnect: disconnectWebSocket,
    getChildren,
    startDownload,
    stopOperation,
  };
}
