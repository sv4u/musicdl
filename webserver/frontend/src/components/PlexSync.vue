<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-3">
        <svg class="w-6 h-6 text-amber-400" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
          <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" />
        </svg>
        <h2 class="text-xl font-bold text-white">Plex Playlist Sync</h2>
      </div>
      <div class="flex items-center gap-3">
        <div
          :class="['w-3 h-3 rounded-full', statusColor]"
          :aria-label="`Status: ${statusLabel}`"
          role="status"
        ></div>
        <span class="text-slate-400 text-sm">{{ statusLabel }}</span>
      </div>
    </div>

    <!-- Action Buttons -->
    <div class="flex gap-3">
      <button
        :disabled="status.isRunning"
        :class="[
          'px-6 py-3 rounded-lg font-medium transition-all flex items-center gap-2',
          status.isRunning
            ? 'bg-slate-600 text-slate-400 cursor-not-allowed'
            : 'bg-amber-600 hover:bg-amber-500 text-white shadow-lg hover:shadow-amber-500/25'
        ]"
        @click="startSync"
      >
        <svg v-if="status.isRunning" class="w-5 h-5 animate-spin" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" aria-hidden="true">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
        </svg>
        <svg v-else class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
          <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182" />
        </svg>
        {{ status.isRunning ? 'Syncing...' : 'Sync Playlists to Plex' }}
      </button>
      <button
        v-if="status.isRunning"
        class="px-4 py-3 rounded-lg font-medium bg-red-600/20 text-red-400 border border-red-600/30 hover:bg-red-600/30 transition-colors"
        @click="stopSync"
      >
        Stop
      </button>
    </div>

    <!-- Progress -->
    <div v-if="status.isRunning || status.total > 0" class="bg-slate-700 rounded-xl p-6 border border-slate-600">
      <div class="flex items-center justify-between mb-3">
        <span class="text-slate-300 font-medium">
          {{ status.isRunning ? 'Syncing playlists...' : 'Last sync results' }}
        </span>
        <span class="text-slate-400 text-sm">{{ status.progress }} / {{ status.total }}</span>
      </div>
      <div class="w-full bg-slate-600 rounded-full h-2 mb-4" role="progressbar" :aria-valuenow="progressPercent" aria-valuemin="0" aria-valuemax="100">
        <div
          class="h-2 rounded-full transition-all duration-500"
          :class="status.error ? 'bg-red-500' : 'bg-amber-500'"
          :style="{ width: `${progressPercent}%` }"
        ></div>
      </div>
      <div v-if="status.error && !status.isRunning" class="text-red-400 text-sm mb-4 flex items-center gap-2">
        <svg class="w-4 h-4 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
          <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
        </svg>
        {{ status.error }}
      </div>

      <!-- Summary chips -->
      <div v-if="!status.isRunning && status.results.length > 0" class="flex flex-wrap gap-3 mb-4">
        <span v-if="createdCount > 0" class="px-3 py-1.5 rounded-lg text-sm font-medium bg-green-600/20 text-green-400 border border-green-600/30">
          {{ createdCount }} created
        </span>
        <span v-if="updatedCount > 0" class="px-3 py-1.5 rounded-lg text-sm font-medium bg-blue-600/20 text-blue-400 border border-blue-600/30">
          {{ updatedCount }} updated
        </span>
        <span v-if="failedCount > 0" class="px-3 py-1.5 rounded-lg text-sm font-medium bg-red-600/20 text-red-400 border border-red-600/30">
          {{ failedCount }} failed
        </span>
      </div>
    </div>

    <!-- Results Table -->
    <div v-if="status.results.length > 0" class="bg-slate-700 rounded-xl border border-slate-600 overflow-hidden">
      <h3 class="text-slate-400 text-xs uppercase tracking-wider px-6 py-4 border-b border-slate-600">
        Playlist Results
      </h3>
      <div class="max-h-96 overflow-y-auto">
        <table class="w-full text-sm">
          <thead class="bg-slate-800/50 sticky top-0">
            <tr>
              <th class="text-left text-slate-400 font-medium px-6 py-3">Playlist</th>
              <th class="text-left text-slate-400 font-medium px-4 py-3">Action</th>
              <th class="text-left text-slate-400 font-medium px-4 py-3">Details</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-600/50">
            <tr v-for="(result, idx) in status.results" :key="idx" class="hover:bg-slate-600/20">
              <td class="px-6 py-3 text-white font-medium">{{ result.playlistName }}</td>
              <td class="px-4 py-3">
                <span :class="actionBadgeClass(result.action)">
                  {{ result.action }}
                </span>
              </td>
              <td class="px-4 py-3 text-slate-400">
                <span v-if="result.error" class="text-red-400">{{ result.error }}</span>
                <span v-else-if="result.trackCount > 0">{{ result.trackCount }} tracks</span>
                <span v-else>—</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- No playlists / not configured -->
    <div v-if="!status.isRunning && status.total === 0 && !configError" class="bg-slate-700 rounded-xl p-8 border border-slate-600 text-center">
      <p class="text-slate-300 text-lg mb-2">Ready to sync</p>
      <p class="text-slate-500 text-sm">
        Click "Sync Playlists to Plex" to push all M3U playlists from your music library to your Plex server.
        Existing playlists will be updated; new ones will be created.
      </p>
    </div>
    <div v-if="configError" class="bg-red-900/20 rounded-xl p-6 border border-red-600/30">
      <div class="flex items-start gap-3">
        <svg class="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
          <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
        </svg>
        <div>
          <p class="text-red-400 font-medium">Configuration Required</p>
          <p class="text-red-300/70 text-sm mt-1">{{ configError }}</p>
          <p class="text-slate-500 text-sm mt-3">
            Add the following to your <code class="text-slate-400">config.yaml</code>:
          </p>
          <pre class="text-slate-400 text-xs mt-2 bg-slate-800 rounded p-3 overflow-x-auto">plex:
  server_url: "http://your-plex-server:32400"
  token: "your-plex-token"
  music_path: "/data/Music"</pre>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import axios from 'axios';

interface PlexSyncResult {
  playlistName: string;
  m3uPath: string;
  action: string;
  error: string;
  trackCount: number;
}

interface PlexSyncStatus {
  isRunning: boolean;
  startedAt: number;
  completedAt: number;
  progress: number;
  total: number;
  error: string;
  results: PlexSyncResult[];
}

const status = ref<PlexSyncStatus>({
  isRunning: false,
  startedAt: 0,
  completedAt: 0,
  progress: 0,
  total: 0,
  error: '',
  results: [],
});

const configError = ref('');
let pollTimer: ReturnType<typeof setTimeout> | null = null;
let unmounted = false;

const progressPercent = computed(() => {
  if (status.value.total === 0) return 0;
  return Math.min(100, (status.value.progress / status.value.total) * 100);
});

const createdCount = computed(() => status.value.results.filter(r => r.action === 'created').length);
const updatedCount = computed(() => status.value.results.filter(r => r.action === 'updated').length);
const failedCount = computed(() => status.value.results.filter(r => r.action === 'failed').length);

const statusLabel = computed(() => {
  if (status.value.isRunning) return 'Syncing';
  if (status.value.error) return 'Error';
  if (status.value.completedAt > 0) return 'Complete';
  return 'Idle';
});

const statusColor = computed(() => {
  if (status.value.isRunning) return 'bg-amber-400 animate-pulse';
  if (status.value.error) return 'bg-red-400';
  if (status.value.completedAt > 0 && failedCount.value === 0) return 'bg-green-400';
  if (status.value.completedAt > 0 && failedCount.value > 0) return 'bg-yellow-400';
  return 'bg-slate-500';
});

function actionBadgeClass(action: string): string {
  const base = 'px-2 py-0.5 rounded text-xs font-medium';
  switch (action) {
    case 'created': return `${base} bg-green-600/20 text-green-400`;
    case 'updated': return `${base} bg-blue-600/20 text-blue-400`;
    case 'failed':  return `${base} bg-red-600/20 text-red-400`;
    default:        return `${base} bg-slate-600/20 text-slate-400`;
  }
}

async function startSync() {
  configError.value = '';
  try {
    await axios.post('/api/plex/sync');
    pollStatus();
  } catch (error) {
    if (axios.isAxiosError(error) && error.response?.data?.error) {
      configError.value = error.response.data.error;
    } else {
      configError.value = 'Failed to start Plex sync';
    }
  }
}

async function stopSync() {
  try {
    await axios.post('/api/plex/stop');
  } catch {
    // Ignore stop errors
  }
}

async function pollStatus() {
  if (pollTimer) {
    clearTimeout(pollTimer);
    pollTimer = null;
  }
  try {
    const response = await axios.get<PlexSyncStatus>('/api/plex/status');
    status.value = response.data;
  } catch {
    // Ignore poll errors
  }
  if (!unmounted) {
    const interval = status.value.isRunning ? 1000 : 5000;
    pollTimer = setTimeout(pollStatus, interval);
  }
}

onMounted(() => {
  pollStatus();
});

onUnmounted(() => {
  unmounted = true;
  if (pollTimer) {
    clearTimeout(pollTimer);
    pollTimer = null;
  }
});
</script>
