<template>
  <div class="space-y-6">
    <Transition name="fade" mode="out-in">
      <!-- Idle / Complete State -->
      <div
        v-if="phase === 'idle' || phase === 'complete' || phase === 'ready'"
        key="idle"
      >
        <!-- Status badge -->
        <div class="flex items-center gap-3 mb-6" aria-live="polite">
          <div
            class="w-3 h-3 rounded-full bg-green-400"
            aria-label="Status: ready"
            role="status"
          ></div>
          <span class="text-slate-300 text-lg font-medium">
            {{ phase === "complete" ? "Download Complete" : "Ready" }}
          </span>
        </div>

        <!-- Last run card -->
        <div
          v-if="lastRun"
          class="bg-slate-700 rounded-xl p-6 border border-slate-600"
        >
          <h3 class="text-slate-400 text-xs uppercase tracking-wider mb-4">
            Last Run
          </h3>
          <p class="text-slate-300 text-sm mb-4">
            {{ formatRunDate(lastRun.startedAt) }}
          </p>

          <div class="grid grid-cols-3 gap-4 mb-4">
            <div class="text-center">
              <div class="text-2xl font-bold text-green-400">
                {{ lastRunStats.downloaded }}
              </div>
              <div class="text-slate-500 text-xs mt-1">downloaded</div>
            </div>
            <div class="text-center">
              <div
                class="text-2xl font-bold"
                :class="
                  lastRunStats.failed > 0 ? 'text-red-400' : 'text-slate-500'
                "
              >
                {{ lastRunStats.failed }}
              </div>
              <div class="text-slate-500 text-xs mt-1">failed</div>
            </div>
            <div class="text-center">
              <div class="text-2xl font-bold text-slate-400">
                {{ lastRunStats.skipped }}
              </div>
              <div class="text-slate-500 text-xs mt-1">skipped</div>
            </div>
          </div>

          <div
            v-if="lastRunStats.elapsedSec > 0"
            class="text-slate-500 text-sm"
          >
            Duration: {{ formatDuration(lastRunStats.elapsedSec) }}
          </div>
        </div>

        <!-- No history -->
        <div
          v-else
          class="bg-slate-700 rounded-xl p-8 border border-slate-600 text-center"
        >
          <p class="text-slate-300 text-lg mb-2">No downloads yet</p>
          <p class="text-slate-500 text-sm">
            Go to the
            <button
              class="text-blue-400 hover:text-blue-300 underline"
              @click="$emit('switchTab', 'Downloads')"
            >
              Downloads
            </button>
            tab to start.
          </p>
        </div>

        <!-- Lifetime stats -->
        <div
          v-if="lifetimeStats.totalDownloaded > 0"
          class="text-slate-500 text-sm mt-4 flex items-center gap-2"
        >
          <span>Lifetime:</span>
          <span class="text-slate-300 font-medium"
            >{{ lifetimeStats.totalDownloaded.toLocaleString() }} songs</span
          >
          <span class="text-slate-600">·</span>
          <span
            :class="
              lifetimeStats.successRate >= 90
                ? 'text-green-400'
                : lifetimeStats.successRate >= 70
                  ? 'text-yellow-400'
                  : 'text-red-400'
            "
          >
            {{ lifetimeStats.successRate.toFixed(1) }}% success
          </span>
        </div>
      </div>

      <!-- Generating State -->
      <div v-else-if="phase === 'generating'" key="generating">
        <div class="flex items-center gap-3 mb-6" aria-live="polite">
          <div
            class="w-3 h-3 rounded-full bg-blue-400 animate-pulse"
            aria-label="Status: generating plan"
            role="status"
          ></div>
          <span class="text-slate-300 text-lg font-medium"
            >Preparing download...</span
          >
        </div>

        <div class="bg-slate-700 rounded-xl p-6 border border-slate-600">
          <div class="flex items-center gap-4">
            <svg
              class="w-8 h-8 text-blue-400 animate-spin flex-shrink-0"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
              aria-hidden="true"
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
            <div>
              <p class="text-white font-medium">
                {{ generationProgress || "Scanning playlists..." }}
              </p>
              <p
                v-if="generationItemsFound > 0"
                class="text-slate-400 text-sm mt-1"
              >
                {{ generationItemsFound }} items found
              </p>
            </div>
          </div>
        </div>
      </div>

      <!-- Downloading State -->
      <div v-else-if="phase === 'downloading'" key="downloading">
        <div class="flex items-center gap-3 mb-6" aria-live="polite">
          <div
            class="w-3 h-3 rounded-full bg-blue-400 animate-pulse"
            aria-label="Status: downloading"
            role="status"
          ></div>
          <span class="text-slate-300 text-lg font-medium">Downloading</span>
        </div>

        <!-- Progress ring + count -->
        <div class="bg-slate-700 rounded-xl p-8 border border-slate-600">
          <div
            class="flex flex-col sm:flex-row items-center justify-center gap-6 sm:gap-10"
          >
            <div
              class="relative"
              :aria-label="`Download progress: ${Math.round(clampedProgress)} percent`"
              role="progressbar"
              :aria-valuenow="Math.round(clampedProgress)"
              aria-valuemin="0"
              aria-valuemax="100"
            >
              <svg viewBox="0 0 120 120" class="w-28 h-28 sm:w-36 sm:h-36">
                <circle
                  cx="60"
                  cy="60"
                  r="52"
                  stroke-width="8"
                  class="fill-none stroke-slate-600"
                />
                <circle
                  cx="60"
                  cy="60"
                  r="52"
                  stroke-width="8"
                  class="fill-none stroke-blue-500"
                  :stroke-dasharray="circumference"
                  :stroke-dashoffset="
                    circumference - (clampedProgress / 100) * circumference
                  "
                  stroke-linecap="round"
                  transform="rotate(-90 60 60)"
                  style="transition: stroke-dashoffset 0.5s ease"
                />
              </svg>
              <div class="absolute inset-0 flex items-center justify-center">
                <span class="text-xl sm:text-2xl font-bold text-white"
                  >{{ Math.round(clampedProgress) }}%</span
                >
              </div>
            </div>

            <div class="text-center">
              <div class="text-2xl sm:text-4xl font-bold text-white">
                {{ planStats.completed + planStats.failed + planStats.skipped }}
                <span class="text-slate-500 text-lg sm:text-2xl font-normal"
                  >/ {{ planStats.total }}</span
                >
              </div>
              <div class="text-slate-400 text-sm mt-1">songs</div>
            </div>
          </div>

          <div
            class="flex flex-wrap justify-center gap-4 sm:gap-6 mt-6 text-sm text-slate-400"
          >
            <span class="flex items-center">
              Speed:
              <span class="text-white font-medium ml-1"
                >{{ currentSpeed.toFixed(1) }} songs/min</span
              >
              <InfoTooltip
                text="Average download speed based on recent completions"
              />
            </span>
            <span v-if="currentEta"
              >ETA:
              <span class="text-white font-medium">{{ currentEta }}</span></span
            >
            <span
              >Elapsed:
              <span class="text-white font-medium">{{
                formatDuration(currentElapsed)
              }}</span></span
            >
          </div>
        </div>

        <!-- Now Playing -->
        <div
          v-if="currentActiveDownloads.length > 0"
          class="bg-slate-700 rounded-xl p-6 border border-slate-600"
        >
          <h3 class="text-slate-400 text-xs uppercase tracking-wider mb-3">
            Now Playing
          </h3>
          <ul class="space-y-2">
            <li
              v-for="item in currentActiveDownloads.slice(0, 3)"
              :key="item.item_id"
              class="flex items-center gap-3 text-slate-300"
            >
              <svg
                class="w-4 h-4 text-blue-400 animate-spin flex-shrink-0"
                xmlns="http://www.w3.org/2000/svg"
                fill="none"
                viewBox="0 0 24 24"
                aria-hidden="true"
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
              <span class="text-white font-medium truncate">{{
                item.name
              }}</span>
              <span
                v-if="getArtist(item)"
                class="text-slate-500 text-sm truncate"
                >{{ getArtist(item) }}</span
              >
            </li>
            <li
              v-if="currentActiveDownloads.length > 3"
              class="text-slate-500 text-sm pl-7"
            >
              +{{ currentActiveDownloads.length - 3 }} more
            </li>
          </ul>
        </div>

        <!-- Current Playlist/Album -->
        <div
          v-if="currentContainer"
          class="bg-slate-700 rounded-xl p-6 border border-slate-600"
        >
          <h3 class="text-slate-400 text-xs uppercase tracking-wider mb-3">
            {{
              currentContainer.item_type === "album"
                ? "Current Album"
                : "Current Playlist"
            }}
          </h3>
          <p class="text-white font-medium text-lg">
            {{ currentContainer.name }}
          </p>
          <div class="mt-2 flex items-center gap-3">
            <div class="flex-1 bg-slate-600 rounded-full h-1.5">
              <div
                class="bg-blue-500 h-1.5 rounded-full transition-all"
                :style="{ width: `${containerProgress}%` }"
              ></div>
            </div>
            <span class="text-slate-400 text-sm whitespace-nowrap">
              {{ containerCompleted }} of {{ containerTotal }} tracks
            </span>
          </div>
        </div>

        <!-- Status chips -->
        <div class="flex flex-wrap gap-3">
          <span
            v-if="planStats.completed > 0"
            class="px-4 py-2 rounded-lg text-sm font-medium bg-green-600/20 text-green-400 border border-green-600/30"
          >
            {{ planStats.completed }} done
          </span>
          <span
            v-if="planStats.failed > 0"
            class="px-4 py-2 rounded-lg text-sm font-medium bg-red-600/20 text-red-400 border border-red-600/30"
          >
            {{ planStats.failed }} failed
          </span>
          <span
            v-if="planStats.skipped > 0"
            class="px-4 py-2 rounded-lg text-sm font-medium bg-yellow-600/20 text-yellow-400 border border-yellow-600/30"
          >
            {{ planStats.skipped }} skipped
          </span>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import axios from "axios";
import InfoTooltip from "./InfoTooltip.vue";
import { usePlanStore } from "../composables/usePlanStore";
import type { PlanItemSnapshot } from "../composables/usePlanStore";

const emit = defineEmits<{
  (e: "switchTab", tab: string): void;
}>();

const {
  phase,
  stats: planStats,
  activeDownloads: currentActiveDownloads,
  rootNodes,
  speed: currentSpeed,
  eta: currentEta,
  elapsedSeconds: currentElapsed,
  progressPercent,
  generationProgress,
  generationItemsFound,
  connect,
  disconnect,
  getChildren,
} = usePlanStore();

const circumference = 2 * Math.PI * 52;
const clampedProgress = computed(() =>
  Math.min(100, Math.max(0, progressPercent.value)),
);

// --- Last run data ---
interface HistoryRunSummary {
  runId: string;
  startedAt: string;
  completedAt?: string;
  state: string;
  statistics: Record<string, number>;
  error?: string;
}

const lastRun = ref<HistoryRunSummary | null>(null);
const lastRunStats = computed(() => {
  if (!lastRun.value?.statistics)
    return { downloaded: 0, failed: 0, skipped: 0, elapsedSec: 0 };
  const s = lastRun.value.statistics;
  return {
    downloaded: s.downloaded ?? 0,
    failed: s.failed ?? 0,
    skipped: s.skipped ?? 0,
    elapsedSec: s.elapsedSec ?? 0,
  };
});

// --- Lifetime stats ---
interface CumulativeStats {
  totalDownloaded: number;
  totalFailed: number;
  successRate: number;
}

const lifetimeStats = ref<CumulativeStats>({
  totalDownloaded: 0,
  totalFailed: 0,
  successRate: 0,
});

// --- Current container ---
const containerTypes = new Set(["playlist", "album", "artist", "m3u"]);

const currentContainer = computed<PlanItemSnapshot | null>(() => {
  for (const root of rootNodes.value) {
    if (!containerTypes.has(root.item_type)) continue;
    const children = getChildren(root.item_id);
    const hasActive = children.some(
      (c) => c.status === "in_progress" || c.status === "pending",
    );
    if (hasActive) return root;
  }
  return rootNodes.value.find((r) => containerTypes.has(r.item_type)) ?? null;
});

const containerCompleted = computed(() => {
  if (!currentContainer.value) return 0;
  const children = getChildren(currentContainer.value.item_id);
  return children.filter(
    (c) => c.item_type === "track" && c.status === "completed",
  ).length;
});

const containerTotal = computed(() => {
  if (!currentContainer.value) return 0;
  const children = getChildren(currentContainer.value.item_id);
  return children.filter((c) => c.item_type === "track").length;
});

const containerProgress = computed(() => {
  if (containerTotal.value === 0) return 0;
  return (containerCompleted.value / containerTotal.value) * 100;
});

// --- Polling ---
let statsTimer: ReturnType<typeof setTimeout> | null = null;
let unmounted = false;

onMounted(() => {
  connect();
  fetchLastRun();
  pollLifetimeStats();
});

onUnmounted(() => {
  unmounted = true;
  disconnect();
  if (statsTimer) {
    clearTimeout(statsTimer);
    statsTimer = null;
  }
});

async function fetchLastRun() {
  try {
    const response = await axios.get<{
      runs: HistoryRunSummary[];
      totalRuns: number;
    }>("/api/history/runs", {
      params: { limit: 1 },
    });
    if (response.data.runs?.length > 0) {
      lastRun.value = response.data.runs[0];
    }
  } catch {
    // History endpoint may not be available yet
  }
}

async function pollLifetimeStats() {
  try {
    const response = await axios.get("/api/stats");
    if (response.data.cumulative) {
      lifetimeStats.value = {
        totalDownloaded: response.data.cumulative.totalDownloaded ?? 0,
        totalFailed: response.data.cumulative.totalFailed ?? 0,
        successRate: response.data.cumulative.successRate ?? 0,
      };
    }
  } catch {
    // Ignore
  }
  if (!unmounted) {
    statsTimer = setTimeout(pollLifetimeStats, 10000);
  }
}

// --- Helpers ---
function getArtist(item: PlanItemSnapshot): string {
  const meta = item.metadata;
  if (!meta) return "";
  const a = (meta.artist ?? meta.artists) as string | string[] | undefined;
  if (typeof a === "string") return a;
  if (Array.isArray(a)) return a.join(", ");
  return "";
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`;
  if (seconds < 3600) {
    const m = Math.floor(seconds / 60);
    const s = Math.round(seconds % 60);
    return `${m}m ${s}s`;
  }
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  return `${h}h ${m}m`;
}

function formatRunDate(dateStr: string): string {
  try {
    const d = new Date(dateStr);
    return d.toLocaleDateString(undefined, {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return dateStr;
  }
}
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.25s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
