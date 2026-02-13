<template>
  <div class="space-y-6">
    <!-- Current Run Stats -->
    <div v-if="currentRun.isRunning" class="bg-slate-700 rounded-lg p-6 border border-slate-600">
      <h3 class="text-white font-semibold mb-4">Current Run</h3>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard label="Downloaded" :value="currentRun.downloaded" color="green" />
        <StatCard label="Failed" :value="currentRun.failed" color="red" />
        <StatCard label="Skipped" :value="currentRun.skipped" color="yellow" />
        <StatCard label="Retries" :value="currentRun.retries" color="blue" />
      </div>
      <div class="mt-4 grid grid-cols-2 md:grid-cols-3 gap-4">
        <div class="text-slate-400 text-sm">
          <span class="text-slate-500">Elapsed:</span> {{ formatDuration(currentRun.elapsedSec) }}
        </div>
        <div class="text-slate-400 text-sm">
          <span class="text-slate-500">Speed:</span> {{ currentRun.tracksPerHour.toFixed(1) }} tracks/hr
        </div>
        <div class="text-slate-400 text-sm">
          <span class="text-slate-500">Written:</span> {{ formatBytes(currentRun.bytesWritten) }}
        </div>
      </div>
    </div>

    <!-- Cumulative Stats -->
    <div class="bg-slate-700 rounded-lg p-6 border border-slate-600">
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-white font-semibold">Lifetime Statistics</h3>
        <button
          @click="resetStats"
          class="text-sm text-red-400 hover:text-red-300 transition-colors"
        >
          Reset
        </button>
      </div>

      <!-- Success Rate Bar -->
      <div class="mb-6">
        <div class="flex items-center justify-between mb-2">
          <span class="text-slate-400 text-sm">Success Rate</span>
          <span class="text-white font-bold">{{ cumulative.successRate.toFixed(1) }}%</span>
        </div>
        <div class="w-full bg-slate-600 rounded-full h-3">
          <div
            class="h-3 rounded-full transition-all"
            :class="cumulative.successRate >= 90 ? 'bg-green-500' : cumulative.successRate >= 70 ? 'bg-yellow-500' : 'bg-red-500'"
            :style="{ width: `${Math.min(100, cumulative.successRate)}%` }"
          ></div>
        </div>
      </div>

      <!-- Stats Grid -->
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <StatCard label="Total Downloaded" :value="cumulative.totalDownloaded" color="green" />
        <StatCard label="Total Failed" :value="cumulative.totalFailed" color="red" />
        <StatCard label="Total Skipped" :value="cumulative.totalSkipped" color="yellow" />
        <StatCard label="Total Runs" :value="cumulative.totalRuns" color="blue" />
      </div>

      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <StatCard label="Plans Generated" :value="cumulative.totalPlansGenerated" color="purple" />
        <StatCard label="Rate Limits" :value="cumulative.totalRateLimits" color="orange" />
        <StatCard label="Retries" :value="cumulative.totalRetries" color="indigo" />
        <div class="bg-slate-800 rounded-lg p-3 border border-slate-600">
          <div class="text-slate-500 text-xs uppercase tracking-wider">Storage Used</div>
          <div class="text-white text-lg font-bold mt-1">{{ formatBytes(cumulative.totalBytesWritten) }}</div>
        </div>
      </div>

      <!-- Time Stats -->
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div class="bg-slate-800 rounded-lg p-3 border border-slate-600">
          <div class="text-slate-500 text-xs uppercase tracking-wider">Total Time</div>
          <div class="text-white text-lg font-bold mt-1">{{ formatDuration(cumulative.totalTimeSpentSec) }}</div>
        </div>
        <div class="bg-slate-800 rounded-lg p-3 border border-slate-600">
          <div class="text-slate-500 text-xs uppercase tracking-wider">Plan Time</div>
          <div class="text-white text-lg font-bold mt-1">{{ formatDuration(cumulative.planTimeSpentSec) }}</div>
        </div>
        <div class="bg-slate-800 rounded-lg p-3 border border-slate-600">
          <div class="text-slate-500 text-xs uppercase tracking-wider">Download Time</div>
          <div class="text-white text-lg font-bold mt-1">{{ formatDuration(cumulative.downloadTimeSpentSec) }}</div>
        </div>
      </div>

      <!-- First/Last Run -->
      <div v-if="cumulative.firstRunAt > 0" class="mt-4 text-slate-500 text-xs">
        First run: {{ formatDate(cumulative.firstRunAt) }} | Last run: {{ formatDate(cumulative.lastRunAt) }}
      </div>
    </div>

    <!-- Recovery Status -->
    <div class="bg-slate-700 rounded-lg p-6 border border-slate-600">
      <h3 class="text-white font-semibold mb-4">Error Recovery</h3>

      <!-- Circuit Breaker -->
      <div class="flex items-center justify-between mb-4">
        <div class="flex items-center gap-3">
          <div
            :class="[
              'w-3 h-3 rounded-full',
              circuitBreaker.state === 'closed' ? 'bg-green-400' :
              circuitBreaker.state === 'half_open' ? 'bg-yellow-400' : 'bg-red-400'
            ]"
          ></div>
          <span class="text-slate-300">
            Circuit Breaker: <span class="font-mono">{{ circuitBreaker.state }}</span>
          </span>
          <span class="text-slate-500 text-sm">
            ({{ circuitBreaker.failureCount }}/{{ circuitBreaker.failureThreshold }} failures)
          </span>
        </div>
        <button
          v-if="circuitBreaker.state !== 'closed'"
          @click="resetCircuitBreaker"
          class="text-sm bg-blue-600 hover:bg-blue-700 text-white py-1 px-3 rounded transition-colors"
        >
          Reset
        </button>
      </div>

      <!-- Resume State -->
      <div v-if="resume.hasResumeData" class="mt-4 bg-slate-800 rounded-lg p-4 border border-slate-600">
        <div class="flex items-center justify-between mb-2">
          <span class="text-slate-300 font-medium">Resume Data Available</span>
          <div class="flex gap-2">
            <button
              @click="retryFailed"
              class="text-sm bg-yellow-600 hover:bg-yellow-700 text-white py-1 px-3 rounded transition-colors"
            >
              Retry Failed ({{ resume.failedCount }})
            </button>
            <button
              @click="clearResume"
              class="text-sm bg-red-600 hover:bg-red-700 text-white py-1 px-3 rounded transition-colors"
            >
              Clear
            </button>
          </div>
        </div>
        <div class="grid grid-cols-3 gap-2 text-sm">
          <span class="text-green-400">Completed: {{ resume.completedCount }}</span>
          <span class="text-red-400">Failed: {{ resume.failedCount }}</span>
          <span class="text-slate-400">Remaining: {{ resume.remainingCount }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import axios from 'axios';
import StatCard from './StatCard.vue';

interface CumulativeStats {
  totalDownloaded: number;
  totalFailed: number;
  totalSkipped: number;
  totalPlansGenerated: number;
  totalRuns: number;
  totalRateLimits: number;
  totalRetries: number;
  totalBytesWritten: number;
  totalTimeSpentSec: number;
  planTimeSpentSec: number;
  downloadTimeSpentSec: number;
  firstRunAt: number;
  lastRunAt: number;
  successRate: number;
}

interface RunStats {
  runId: string;
  operationType: string;
  startedAt: number;
  downloaded: number;
  failed: number;
  skipped: number;
  retries: number;
  rateLimits: number;
  bytesWritten: number;
  elapsedSec: number;
  tracksPerHour: number;
  isRunning: boolean;
}

interface CircuitBreakerStatus {
  state: string;
  failureCount: number;
  successCount: number;
  failureThreshold: number;
  successThreshold: number;
  resetTimeoutSec: number;
  lastFailureAt: number;
  lastStateChange: number;
  canRetry: boolean;
}

interface ResumeStatus {
  hasResumeData: boolean;
  completedCount: number;
  failedCount: number;
  totalItems: number;
  remainingCount: number;
}

const cumulative = ref<CumulativeStats>({
  totalDownloaded: 0, totalFailed: 0, totalSkipped: 0, totalPlansGenerated: 0,
  totalRuns: 0, totalRateLimits: 0, totalRetries: 0, totalBytesWritten: 0,
  totalTimeSpentSec: 0, planTimeSpentSec: 0, downloadTimeSpentSec: 0,
  firstRunAt: 0, lastRunAt: 0, successRate: 0,
});

const currentRun = ref<RunStats>({
  runId: '', operationType: '', startedAt: 0, downloaded: 0,
  failed: 0, skipped: 0, retries: 0, rateLimits: 0,
  bytesWritten: 0, elapsedSec: 0, tracksPerHour: 0, isRunning: false,
});

const circuitBreaker = ref<CircuitBreakerStatus>({
  state: 'closed', failureCount: 0, successCount: 0,
  failureThreshold: 5, successThreshold: 3, resetTimeoutSec: 60,
  lastFailureAt: 0, lastStateChange: 0, canRetry: true,
});

const resume = ref<ResumeStatus>({
  hasResumeData: false, completedCount: 0, failedCount: 0,
  totalItems: 0, remainingCount: 0,
});

let pollInterval: ReturnType<typeof setInterval> | null = null;

onMounted(() => {
  fetchAll();
  pollInterval = setInterval(fetchAll, 3000);
});

onUnmounted(() => {
  if (pollInterval) clearInterval(pollInterval);
});

async function fetchAll() {
  await Promise.all([fetchStats(), fetchRecovery()]);
}

async function fetchStats() {
  try {
    const response = await axios.get('/api/stats');
    if (response.data.cumulative) cumulative.value = response.data.cumulative;
    if (response.data.currentRun) currentRun.value = response.data.currentRun;
  } catch { /* ignore */ }
}

async function fetchRecovery() {
  try {
    const response = await axios.get('/api/recovery/status');
    if (response.data.circuitBreaker) circuitBreaker.value = response.data.circuitBreaker;
    if (response.data.resume) resume.value = response.data.resume;
  } catch { /* ignore */ }
}

async function resetStats() {
  if (!confirm('Reset all lifetime statistics?')) return;
  try {
    await axios.post('/api/stats/reset');
    await fetchStats();
  } catch { /* ignore */ }
}

async function resetCircuitBreaker() {
  try {
    await axios.post('/api/recovery/circuit-breaker/reset');
    await fetchRecovery();
  } catch { /* ignore */ }
}

async function retryFailed() {
  try {
    await axios.post('/api/recovery/resume/retry-failed');
    await fetchRecovery();
  } catch { /* ignore */ }
}

async function clearResume() {
  try {
    await axios.post('/api/recovery/resume/clear');
    await fetchRecovery();
  } catch { /* ignore */ }
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${Math.round(seconds % 60)}s`;
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  return `${h}h ${m}m`;
}

function formatDate(timestamp: number): string {
  if (timestamp <= 0) return 'N/A';
  return new Date(timestamp * 1000).toLocaleDateString(undefined, {
    year: 'numeric', month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  });
}
</script>
