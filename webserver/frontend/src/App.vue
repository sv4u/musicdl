<template>
  <div id="app" class="min-h-screen bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900">
    <!-- Header -->
    <header class="bg-slate-900 border-b border-slate-700 shadow-lg">
      <div class="max-w-7xl mx-auto px-4 py-6 flex items-center justify-between">
        <div class="flex items-center gap-4">
          <h1 class="text-3xl font-bold text-white flex items-center gap-2">
            <svg class="w-8 h-8 text-blue-400" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
              <path d="M3 2a2 2 0 012-2h10a2 2 0 012 2v16a2 2 0 01-2 2H5a2 2 0 01-2-2V2z" />
            </svg>
            musicdl
            <span v-if="versionInfo.musicdl" class="text-slate-500 font-normal text-base ml-1">{{ versionInfo.musicdl }}</span>
          </h1>
          <span v-if="versionInfo.spotigo" class="text-slate-500 text-sm border-l border-slate-600 pl-4">spotigo {{ versionInfo.spotigo }}</span>
        </div>
        <a
          href="/api/docs"
          target="_blank"
          class="text-slate-400 hover:text-blue-400 text-sm transition-colors flex items-center gap-1"
        >
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z" clip-rule="evenodd" />
          </svg>
          API Docs
        </a>
      </div>
    </header>

    <!-- Main Content -->
    <main class="max-w-7xl mx-auto px-4 py-8">
      <!-- Status Cards -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-8">
        <!-- API Health -->
        <div class="bg-slate-800 rounded-lg p-6 border border-slate-700">
          <h2 class="text-lg font-semibold text-white mb-4">API Status</h2>
          <div class="flex items-center gap-3">
            <div :class="['w-4 h-4 rounded-full', apiHealthy ? 'bg-green-400' : 'bg-red-400']" :aria-label="apiHealthy ? 'API connected' : 'API disconnected'" role="status"></div>
            <span class="text-slate-300">{{ apiHealthy ? 'Connected' : 'Disconnected' }}</span>
            <span v-if="wsClients > 0" class="text-slate-500 text-sm ml-auto flex items-center">
              {{ wsClients }} WS client(s)
              <InfoTooltip text="Active WebSocket connections to this dashboard for live updates" />
            </span>
          </div>
        </div>

        <!-- Rate Limit Status -->
        <RateLimitAlert v-if="rateLimitInfo.active" :info="rateLimitInfo" />
        <div v-else class="bg-slate-800 rounded-lg p-6 border border-slate-700">
          <h2 class="text-lg font-semibold text-white mb-4">Spotify Rate Limit</h2>
          <div class="flex items-center gap-3">
            <div class="w-4 h-4 rounded-full bg-green-400" aria-label="No active rate limit" role="status"></div>
            <span class="text-slate-300">No active rate limit</span>
          </div>
        </div>
      </div>

      <!-- Tabs -->
      <div class="bg-slate-800 rounded-lg border border-slate-700 shadow-lg">
        <div class="flex border-b border-slate-700 overflow-x-auto scrollbar-hide" role="tablist" aria-label="Dashboard navigation">
          <button
            v-for="tab in tabs"
            :key="tab"
            role="tab"
            :aria-selected="activeTab === tab"
            :aria-controls="`tabpanel-${tab.toLowerCase()}`"
            :class="['px-6 py-3 font-medium transition-colors whitespace-nowrap flex items-center gap-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400 focus-visible:ring-offset-2 focus-visible:ring-offset-slate-800 rounded-t-lg', activeTab === tab ? 'text-blue-400 border-b-2 border-blue-400' : 'text-slate-400 hover:text-white']"
            @click="activeTab = tab"
          >
            <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" :d="tabIcons[tab]" />
            </svg>
            {{ tab }}
          </button>
        </div>

        <div class="p-8">
          <div v-show="activeTab === 'Overview'" id="tabpanel-overview" role="tabpanel" aria-label="Overview" class="space-y-6">
            <OverviewDashboard @switch-tab="(tab: string) => activeTab = tab as typeof activeTab" />
          </div>

          <div v-show="activeTab === 'Downloads'" id="tabpanel-downloads" role="tabpanel" aria-label="Downloads" class="space-y-6">
            <DownloadDashboard />
            <StatsDashboard />
          </div>

          <div v-show="activeTab === 'Analytics'" id="tabpanel-analytics" role="tabpanel" aria-label="Analytics" class="space-y-6">
            <AnalyticsDashboard />
          </div>

          <div v-show="activeTab === 'Plex'" id="tabpanel-plex" role="tabpanel" aria-label="Plex" class="space-y-6">
            <PlexSync />
          </div>

          <div v-show="activeTab === 'Logs'" id="tabpanel-logs" role="tabpanel" aria-label="Logs" class="space-y-6">
            <LogViewer />
          </div>

          <div v-show="activeTab === 'Configuration'" id="tabpanel-configuration" role="tabpanel" aria-label="Configuration" class="space-y-6">
            <ConfigEditor v-if="configExists" />
            <div v-else class="text-center py-12">
              <p class="text-slate-400">No configuration file found at /download/config.yaml</p>
            </div>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import axios from 'axios';
import OverviewDashboard from './components/OverviewDashboard.vue';
import DownloadDashboard from './components/DownloadDashboard.vue';
import StatsDashboard from './components/StatsDashboard.vue';
import AnalyticsDashboard from './components/AnalyticsDashboard.vue';
import ConfigEditor from './components/ConfigEditor.vue';
import LogViewer from './components/LogViewer.vue';
import RateLimitAlert from './components/RateLimitAlert.vue';
import InfoTooltip from './components/InfoTooltip.vue';
import PlexSync from './components/PlexSync.vue';

const tabIcons: Record<string, string> = {
  Overview: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6',
  Downloads: 'M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4',
  Analytics: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
  Plex: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5',
  Logs: 'M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z',
  Configuration: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z',
};

interface RateLimitInfo {
  active: boolean;
  retryAfterSeconds: number;
  retryAfterTimestamp: number;
  detectedAt: number;
  remainingSeconds: number;
}

interface VersionInfo {
  musicdl: string;
  spotigo: string;
}

const activeTab = ref<'Overview' | 'Downloads' | 'Analytics' | 'Plex' | 'Logs' | 'Configuration'>('Overview');
const tabs = ['Overview', 'Downloads', 'Analytics', 'Plex', 'Logs', 'Configuration'];
const apiHealthy = ref(false);
const configExists = ref(false);
const wsClients = ref(0);
const rateLimitInfo = ref<RateLimitInfo>({
  active: false,
  retryAfterSeconds: 0,
  retryAfterTimestamp: 0,
  detectedAt: 0,
  remainingSeconds: 0,
});
const versionInfo = ref<VersionInfo>({ musicdl: '', spotigo: '' });

let healthTimer: ReturnType<typeof setTimeout> | null = null;
let rateLimitTimer: ReturnType<typeof setTimeout> | null = null;
let unmounted = false;

onMounted(() => {
  checkAPIHealth();
  checkConfigExists();
  pollRateLimitStatus();
  fetchVersion();
});

onUnmounted(() => {
  unmounted = true;
  if (healthTimer !== null) clearTimeout(healthTimer);
  if (rateLimitTimer !== null) clearTimeout(rateLimitTimer);
});

async function checkAPIHealth() {
  try {
    const response = await axios.get('/api/health');
    apiHealthy.value = true;
    wsClients.value = response.data.wsClients || 0;
  } catch {
    apiHealthy.value = false;
  }
  if (!unmounted) healthTimer = setTimeout(checkAPIHealth, 10000);
}

async function checkConfigExists() {
  try {
    await axios.get('/api/config');
    configExists.value = true;
  } catch (error) {
    if (axios.isAxiosError(error) && error.response?.status === 404) {
      configExists.value = false;
    }
  }
}

async function fetchVersion() {
  try {
    const response = await axios.get<VersionInfo>('/api/version');
    versionInfo.value = response.data;
  } catch {
    // Leave versionInfo empty on error
  }
}

async function pollRateLimitStatus() {
  try {
    const response = await axios.get('/api/rate-limit-status');
    const now = Math.floor(Date.now() / 1000);
    rateLimitInfo.value = {
      ...response.data,
      remainingSeconds: Math.max(0, response.data.retryAfterTimestamp - now),
    };

    if (!unmounted) {
      rateLimitTimer = setTimeout(pollRateLimitStatus, response.data.active ? 1000 : 5000);
    }
  } catch {
    if (!unmounted) rateLimitTimer = setTimeout(pollRateLimitStatus, 5000);
  }
}
</script>
