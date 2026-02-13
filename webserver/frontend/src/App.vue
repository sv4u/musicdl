<template>
  <div id="app" class="min-h-screen bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900">
    <!-- Header -->
    <header class="bg-slate-900 border-b border-slate-700 shadow-lg">
      <div class="max-w-7xl mx-auto px-4 py-6 flex items-center justify-between">
        <h1 class="text-3xl font-bold text-white flex items-center gap-2">
          <svg class="w-8 h-8 text-blue-400" fill="currentColor" viewBox="0 0 20 20">
            <path d="M3 2a2 2 0 012-2h10a2 2 0 012 2v16a2 2 0 01-2 2H5a2 2 0 01-2-2V2z" />
          </svg>
          musicdl
        </h1>
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
            <div :class="['w-4 h-4 rounded-full', apiHealthy ? 'bg-green-400' : 'bg-red-400']"></div>
            <span class="text-slate-300">{{ apiHealthy ? 'Connected' : 'Disconnected' }}</span>
            <span v-if="wsClients > 0" class="text-slate-500 text-sm ml-auto">{{ wsClients }} WS client(s)</span>
          </div>
        </div>

        <!-- Rate Limit Status -->
        <RateLimitAlert v-if="rateLimitInfo.active" :info="rateLimitInfo" />
        <div v-else class="bg-slate-800 rounded-lg p-6 border border-slate-700">
          <h2 class="text-lg font-semibold text-white mb-4">Spotify Rate Limit</h2>
          <div class="flex items-center gap-3">
            <div class="w-4 h-4 rounded-full bg-green-400"></div>
            <span class="text-slate-300">No active rate limit</span>
          </div>
        </div>
      </div>

      <!-- Tabs -->
      <div class="bg-slate-800 rounded-lg border border-slate-700 shadow-lg">
        <div class="flex border-b border-slate-700 overflow-x-auto">
          <button
            v-for="tab in tabs"
            :key="tab"
            :class="['px-6 py-3 font-medium transition-colors whitespace-nowrap', activeTab === tab ? 'text-blue-400 border-b-2 border-blue-400' : 'text-slate-400 hover:text-white']"
            @click="activeTab = tab"
          >
            {{ tab }}
          </button>
        </div>

        <div class="p-8">
          <!-- Download Tab -->
          <div v-if="activeTab === 'Download'" class="space-y-6">
            <DownloadRunner />
          </div>

          <!-- Configuration Tab -->
          <div v-if="activeTab === 'Configuration'" class="space-y-6">
            <ConfigEditor v-if="configExists" />
            <div v-else class="text-center py-12">
              <p class="text-slate-400">No configuration file found at /download/config.yaml</p>
            </div>
          </div>

          <!-- Logs Tab -->
          <div v-if="activeTab === 'Logs'" class="space-y-6">
            <LogViewer />
          </div>

          <!-- Statistics Tab -->
          <div v-if="activeTab === 'Statistics'" class="space-y-6">
            <StatsDashboard />
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import axios from 'axios';
import DownloadRunner from './components/DownloadRunner.vue';
import ConfigEditor from './components/ConfigEditor.vue';
import LogViewer from './components/LogViewer.vue';
import RateLimitAlert from './components/RateLimitAlert.vue';
import StatsDashboard from './components/StatsDashboard.vue';

interface RateLimitInfo {
  active: boolean;
  retryAfterSeconds: number;
  retryAfterTimestamp: number;
  detectedAt: number;
  remainingSeconds: number;
}

const activeTab = ref<'Download' | 'Configuration' | 'Logs' | 'Statistics'>('Download');
const tabs = ['Download', 'Configuration', 'Logs', 'Statistics'];
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

onMounted(() => {
  checkAPIHealth();
  checkConfigExists();
  pollRateLimitStatus();
});

async function checkAPIHealth() {
  try {
    const response = await axios.get('/api/health');
    apiHealthy.value = true;
    wsClients.value = response.data.wsClients || 0;
  } catch {
    apiHealthy.value = false;
  }
  setTimeout(checkAPIHealth, 10000);
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

async function pollRateLimitStatus() {
  try {
    const response = await axios.get('/api/rate-limit-status');
    const now = Math.floor(Date.now() / 1000);
    rateLimitInfo.value = {
      ...response.data,
      remainingSeconds: Math.max(0, response.data.retryAfterTimestamp - now),
    };

    if (response.data.active) {
      setTimeout(pollRateLimitStatus, 1000);
    } else {
      setTimeout(pollRateLimitStatus, 5000);
    }
  } catch {
    setTimeout(pollRateLimitStatus, 5000);
  }
}
</script>
