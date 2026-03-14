<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-white text-lg font-semibold">Run History Analytics</h2>
        <p class="text-slate-500 text-sm mt-1">
          {{ runs.length }} run{{ runs.length !== 1 ? "s" : "" }} loaded
        </p>
      </div>
      <button
        @click="fetchData"
        :disabled="loading"
        aria-label="Refresh analytics data"
        class="text-sm bg-slate-600 hover:bg-slate-500 disabled:bg-slate-700 text-white py-2 px-4 rounded-lg transition-colors flex items-center gap-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
      >
        <svg
          v-if="loading"
          class="w-4 h-4 animate-spin"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
        >
          <circle
            class="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            stroke-width="4"
          />
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          />
        </svg>
        Refresh
      </button>
    </div>

    <!-- Loading state -->
    <div v-if="loading && runs.length === 0" class="text-center py-12">
      <svg
        class="w-8 h-8 text-blue-400 animate-spin mx-auto mb-4"
        xmlns="http://www.w3.org/2000/svg"
        fill="none"
        viewBox="0 0 24 24"
      >
        <circle
          class="opacity-25"
          cx="12"
          cy="12"
          r="10"
          stroke="currentColor"
          stroke-width="4"
        />
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
        />
      </svg>
      <p class="text-slate-400">Loading analytics...</p>
    </div>

    <!-- Empty state -->
    <div v-else-if="!loading && runs.length === 0" class="text-center py-12">
      <p class="text-slate-400 text-lg mb-2">No run history yet</p>
      <p class="text-slate-500 text-sm">
        Complete a download to see analytics here.
      </p>
    </div>

    <!-- Charts -->
    <template v-else>
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div
          class="bg-slate-700 rounded-xl p-6 border border-slate-600"
          aria-label="Run performance chart showing speed and success rate over time"
        >
          <h3 class="text-white font-semibold mb-4">Run Performance</h3>
          <div class="h-48 sm:h-64">
            <Line
              :data="performanceChartData"
              :options="performanceChartOptions"
            />
          </div>
        </div>

        <div
          class="bg-slate-700 rounded-xl p-6 border border-slate-600"
          aria-label="Bar chart comparing downloaded versus failed tracks per run"
        >
          <h3 class="text-white font-semibold mb-4">Downloaded vs Failed</h3>
          <div class="h-48 sm:h-64">
            <Bar :data="barChartData" :options="barChartOptions" />
          </div>
        </div>

        <div
          class="bg-slate-700 rounded-xl p-6 border border-slate-600"
          aria-label="Doughnut chart showing overall outcome distribution"
        >
          <h3 class="text-white font-semibold mb-4">Outcome Distribution</h3>
          <div class="h-48 sm:h-64 flex items-center justify-center">
            <Doughnut
              :data="doughnutChartData"
              :options="doughnutChartOptions"
            />
          </div>
        </div>

        <div
          class="bg-slate-700 rounded-xl p-6 border border-slate-600"
          aria-label="Line chart showing cumulative storage growth over time"
        >
          <h3 class="text-white font-semibold mb-4">Storage Growth</h3>
          <div class="h-48 sm:h-64">
            <Line :data="storageChartData" :options="storageChartOptions" />
          </div>
        </div>
      </div>

      <!-- Failed Songs Table -->
      <div
        v-if="failedSongs.length > 0"
        class="bg-slate-700 rounded-xl p-6 border border-slate-600"
      >
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-white font-semibold">Failed Songs</h3>
          <input
            v-model="failedSearch"
            placeholder="Search name, artist, error..."
            aria-label="Search failed songs"
            class="bg-slate-800 text-slate-300 border border-slate-600 rounded-lg px-3 py-1.5 text-sm w-48 sm:w-64 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
          />
        </div>
        <div class="overflow-x-auto">
          <table class="w-full text-sm text-left">
            <thead
              class="text-slate-500 text-xs uppercase border-b border-slate-600"
            >
              <tr>
                <th class="py-2 pr-4">Track</th>
                <th class="py-2 pr-4">Error</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(song, idx) in filteredFailedSongs.slice(
                  0,
                  failedShowCount,
                )"
                :key="idx"
                class="border-b border-slate-600/50"
              >
                <td class="py-3 pr-4">
                  <span class="text-white">{{ song.name }}</span>
                  <span v-if="song.artist" class="text-slate-500 ml-2">{{
                    song.artist
                  }}</span>
                </td>
                <td class="py-3 pr-4 text-red-400 text-xs font-mono">
                  {{ song.error }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <button
          v-if="filteredFailedSongs.length > failedShowCount"
          @click="failedShowCount += 50"
          class="mt-3 text-sm text-blue-400 hover:text-blue-300"
        >
          Show more ({{ filteredFailedSongs.length - failedShowCount }}
          remaining)
        </button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import axios from "axios";
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  Filler,
} from "chart.js";
import { Line, Bar, Doughnut } from "vue-chartjs";

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  Filler,
);

ChartJS.defaults.color = "#94A3B8";
ChartJS.defaults.borderColor = "#334155";
ChartJS.defaults.font.family = "system-ui, -apple-system, sans-serif";

interface HistoryRunSummary {
  runId: string;
  startedAt: string;
  completedAt?: string;
  state: string;
  statistics: Record<string, number>;
  error?: string;
  snapshotCount: number;
}

interface RunDataPoint {
  label: string;
  downloaded: number;
  failed: number;
  skipped: number;
  speed: number;
  successRate: number;
  bytesWritten: number;
  cumulativeBytes: number;
}

interface FailedSong {
  name: string;
  artist: string;
  error: string;
}

interface FailedItemInfo {
  url: string;
  name: string;
  error: string;
}

const runs = ref<HistoryRunSummary[]>([]);
const loading = ref(false);
const failedSongs = ref<FailedSong[]>([]);
const failedSearch = ref("");
const failedShowCount = ref(50);

const dataPoints = computed<RunDataPoint[]>(() => {
  const sorted = [...runs.value].reverse();
  let cumBytes = 0;
  return sorted.map((run) => {
    const s = run.statistics ?? {};
    const downloaded = s.downloaded ?? 0;
    const failed = s.failed ?? 0;
    const skipped = s.skipped ?? 0;
    const elapsed = s.elapsedSec ?? 0;
    const bytes = s.bytesWritten ?? 0;
    cumBytes += bytes;
    const total = downloaded + failed;
    return {
      label: formatShortDate(run.startedAt),
      downloaded,
      failed,
      skipped,
      speed: elapsed > 0 ? (downloaded / elapsed) * 3600 : 0,
      successRate: total > 0 ? (downloaded / total) * 100 : 100,
      bytesWritten: bytes,
      cumulativeBytes: cumBytes,
    };
  });
});

const filteredFailedSongs = computed(() => {
  if (!failedSearch.value) return failedSongs.value;
  const q = failedSearch.value.toLowerCase();
  return failedSongs.value.filter(
    (s) =>
      s.name.toLowerCase().includes(q) ||
      s.artist.toLowerCase().includes(q) ||
      s.error.toLowerCase().includes(q),
  );
});

// --- Chart data ---

const performanceChartData = computed(() => ({
  labels: dataPoints.value.map((d) => d.label),
  datasets: [
    {
      label: "Speed (tracks/hr)",
      data: dataPoints.value.map((d) => Math.round(d.speed * 10) / 10),
      borderColor: "#60A5FA",
      backgroundColor: "#60A5FA",
      tension: 0.3,
      pointRadius: 4,
      pointHoverRadius: 6,
      yAxisID: "y",
    },
    {
      label: "Success Rate (%)",
      data: dataPoints.value.map((d) => Math.round(d.successRate * 10) / 10),
      borderColor: "#4ADE80",
      backgroundColor: "#4ADE80",
      tension: 0.3,
      pointRadius: 4,
      pointHoverRadius: 6,
      yAxisID: "y1",
    },
  ],
}));

const performanceChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: "index" as const, intersect: false },
  plugins: { legend: { position: "bottom" as const } },
  scales: {
    y: {
      type: "linear" as const,
      position: "left" as const,
      title: { display: true, text: "Speed (tracks/hr)" },
      grid: { color: "#1E293B" },
    },
    y1: {
      type: "linear" as const,
      position: "right" as const,
      title: { display: true, text: "Success Rate (%)" },
      min: 0,
      max: 100,
      grid: { drawOnChartArea: false },
    },
    x: { grid: { color: "#1E293B" } },
  },
}));

const barChartData = computed(() => ({
  labels: dataPoints.value.map((d) => d.label),
  datasets: [
    {
      label: "Downloaded",
      data: dataPoints.value.map((d) => d.downloaded),
      backgroundColor: "#4ADE80",
      borderRadius: 4,
      maxBarThickness: 40,
    },
    {
      label: "Failed",
      data: dataPoints.value.map((d) => d.failed),
      backgroundColor: "#F87171",
      borderRadius: 4,
      maxBarThickness: 40,
    },
  ],
}));

const barChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { position: "bottom" as const } },
  scales: {
    y: {
      title: { display: true, text: "Tracks" },
      grid: { color: "#1E293B" },
    },
    x: { grid: { color: "#1E293B" } },
  },
}));

const doughnutChartData = computed(() => {
  const totals = dataPoints.value.reduce(
    (acc, d) => ({
      downloaded: acc.downloaded + d.downloaded,
      failed: acc.failed + d.failed,
      skipped: acc.skipped + d.skipped,
    }),
    { downloaded: 0, failed: 0, skipped: 0 },
  );
  return {
    labels: ["Downloaded", "Failed", "Skipped"],
    datasets: [
      {
        data: [totals.downloaded, totals.failed, totals.skipped],
        backgroundColor: ["#4ADE80", "#F87171", "#FACC15"],
        borderColor: "#334155",
        borderWidth: 2,
      },
    ],
  };
});

const doughnutChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  cutout: "60%",
  plugins: {
    legend: { position: "bottom" as const },
  },
}));

const storageChartData = computed(() => ({
  labels: dataPoints.value.map((d) => d.label),
  datasets: [
    {
      label: "Cumulative Storage",
      data: dataPoints.value.map((d) => d.cumulativeBytes),
      borderColor: "#C084FC",
      backgroundColor: "rgba(192, 132, 252, 0.1)",
      tension: 0.3,
      pointRadius: 4,
      pointHoverRadius: 6,
      fill: true,
    },
  ],
}));

const storageChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        label: (ctx: { parsed: { y: number } }) => formatBytes(ctx.parsed.y),
      },
    },
  },
  scales: {
    y: {
      title: { display: true, text: "Storage" },
      grid: { color: "#1E293B" },
      ticks: {
        callback: (value: string | number) => formatBytes(Number(value)),
      },
    },
    x: { grid: { color: "#1E293B" } },
  },
}));

// --- Data fetching ---

onMounted(() => {
  fetchData();
});

async function fetchData() {
  loading.value = true;
  try {
    await Promise.all([fetchRuns(), fetchFailedSongs()]);
  } finally {
    loading.value = false;
  }
}

async function fetchRuns() {
  try {
    const response = await axios.get<{ runs: HistoryRunSummary[] }>(
      "/api/history/runs",
      {
        params: { limit: 20 },
      },
    );
    runs.value = response.data.runs ?? [];
  } catch {
    runs.value = [];
  }
}

async function fetchFailedSongs() {
  try {
    const response = await axios.get("/api/recovery/status");
    const resume = response.data?.resume;
    if (!resume?.hasResumeData || !resume?.failedItems) {
      failedSongs.value = [];
      return;
    }
    const items: Record<string, FailedItemInfo> = resume.failedItems;
    failedSongs.value = Object.values(items).map((item) => ({
      name: item.name ?? "Unknown",
      artist: "",
      error: item.error ?? "Unknown error",
    }));
  } catch {
    failedSongs.value = [];
  }
}

// --- Helpers ---

function formatShortDate(dateStr: string): string {
  try {
    const d = new Date(dateStr);
    return (
      d.toLocaleDateString(undefined, { month: "short", day: "numeric" }) +
      " " +
      d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" })
    );
  } catch {
    return dateStr;
  }
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.min(
    Math.floor(Math.log(bytes) / Math.log(k)),
    sizes.length - 1,
  );
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
}
</script>
