<template>
  <div class="space-y-6">
    <!-- Action buttons row -->
    <div class="flex gap-4 flex-wrap">
      <button
        @click="handleRunDownload"
        :disabled="isRunning"
        class="flex-1 min-w-[140px] bg-green-600 hover:bg-green-700 disabled:bg-green-400 disabled:cursor-not-allowed text-white font-semibold py-3 rounded-lg transition-colors"
      >
        {{ isRunning ? 'Downloading...' : 'Run Download' }}
      </button>
      <button
        v-if="hasResumeData"
        @click="handleResume"
        :disabled="isRunning"
        class="bg-yellow-600 hover:bg-yellow-700 disabled:bg-yellow-400 disabled:cursor-not-allowed text-white font-semibold py-3 px-4 rounded-lg transition-colors"
        title="Resume interrupted download"
      >
        Resume
      </button>
      <button
        v-if="showStop"
        @click="handleStop"
        class="bg-red-600 hover:bg-red-700 text-white font-semibold py-3 px-4 rounded-lg transition-colors"
      >
        Stop
      </button>
    </div>

    <!-- Plan generation progress (phase is generating) -->
    <PlanGenerationProgress
      v-if="phase === 'generating'"
      :message="generationProgress"
      :items-found="generationItemsFound"
    />

    <!-- Plan summary (phase is not idle) -->
    <PlanSummary
      v-if="phase !== 'idle'"
      :stats="stats"
      :eta="eta"
      :speed="speed"
      :elapsed-seconds="elapsedSeconds"
      :progress-percent="progressPercent"
      :phase="phase"
      :rate-limit-info="rateLimitInfo"
    />

    <!-- Active downloads -->
    <ActiveDownloads v-if="activeDownloads.length > 0" :items="activeDownloads" />

    <!-- Failed items panel -->
    <FailedItemsPanel v-if="failedItems.length > 0" :items="failedItems" />

    <!-- Plan tree -->
    <PlanTree v-if="items.size > 0" :root-nodes="rootNodes" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import axios from 'axios';
import { usePlanStore } from '../composables/usePlanStore';
import PlanGenerationProgress from './PlanGenerationProgress.vue';
import PlanSummary from './PlanSummary.vue';
import ActiveDownloads from './ActiveDownloads.vue';
import FailedItemsPanel from './FailedItemsPanel.vue';
import PlanTree from './PlanTree.vue';

const {
  phase,
  items,
  stats,
  activeDownloads,
  failedItems,
  rootNodes,
  eta,
  speed,
  elapsedSeconds,
  progressPercent,
  generationProgress,
  generationItemsFound,
  connect,
  disconnect,
  startDownload,
  stopOperation,
} = usePlanStore();

const hasResumeData = ref(false);
const rateLimitInfo = ref<{ active: boolean; remainingSeconds: number }>({
  active: false,
  remainingSeconds: 0,
});

const isRunning = computed(
  () => phase.value === 'generating' || phase.value === 'downloading'
);

const showStop = computed(
  () => phase.value === 'generating' || phase.value === 'downloading'
);

let rateLimitTimer: ReturnType<typeof setTimeout> | null = null;
let unmounted = false;

onMounted(() => {
  connect();
  checkResumeState();
  pollRateLimitStatus();
});

onUnmounted(() => {
  unmounted = true;
  disconnect();
  if (rateLimitTimer !== null) {
    clearTimeout(rateLimitTimer);
    rateLimitTimer = null;
  }
});

async function checkResumeState() {
  try {
    const response = await axios.get('/api/recovery/status');
    hasResumeData.value = response.data.resume?.hasResumeData ?? false;
  } catch {
    /* ignore */
  }
}

async function pollRateLimitStatus() {
  try {
    const response = await axios.get('/api/rate-limit-status');
    const now = Math.floor(Date.now() / 1000);
    rateLimitInfo.value = {
      active: response.data.active ?? false,
      remainingSeconds: Math.max(0, (response.data.retryAfterTimestamp ?? now) - now),
    };
  } catch {
    rateLimitInfo.value = { active: false, remainingSeconds: 0 };
  }
  if (!unmounted) {
    rateLimitTimer = setTimeout(
      pollRateLimitStatus,
      rateLimitInfo.value.active ? 1000 : 5000
    );
  }
}

async function handleRunDownload() {
  await startDownload(false);
  await checkResumeState();
}

async function handleResume() {
  await startDownload(true);
  await checkResumeState();
}

async function handleStop() {
  await stopOperation();
}
</script>
