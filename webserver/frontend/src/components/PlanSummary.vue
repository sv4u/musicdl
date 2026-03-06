<template>
  <div class="space-y-4">
    <!-- Rate limit inline warning -->
    <div
      v-if="rateLimitInfo?.active"
      class="bg-amber-950/50 border border-amber-700 rounded-lg p-4"
    >
      <p class="text-amber-200 font-medium">
        Rate limited: {{ rateLimitInfo.remainingSeconds }} seconds remaining
        ({{ formatRemainingTime(rateLimitInfo.remainingSeconds) }})
      </p>
    </div>

    <!-- Full view: when generating, parent shows PlanGenerationProgress; we only show rate limit above. -->
    <div
      v-if="phase !== 'generating'"
      class="bg-slate-700 rounded-lg p-6 border border-slate-600 space-y-4"
    >
      <!-- Progress bar -->
      <div>
        <div class="flex justify-between mb-2">
          <span class="text-slate-300 text-sm">
            {{ stats.completed + stats.failed + stats.skipped }}/{{ stats.total }} tracks
            ({{ Math.round(progressPercent) }}%)
          </span>
        </div>
        <div class="w-full bg-slate-600 rounded-full h-2">
          <div
            class="bg-blue-600 h-2 rounded-full transition-all"
            :style="{ width: `${Math.min(100, progressPercent)}%` }"
          />
        </div>
      </div>

      <!-- Stats row -->
      <div class="flex flex-wrap gap-4 text-slate-400 text-sm">
        <span>Elapsed: {{ formatElapsed(elapsedSeconds) }}</span>
        <span v-if="eta">ETA: {{ eta }}</span>
        <span>Speed: {{ speed.toFixed(1) }} tracks/min</span>
      </div>

      <!-- Status chips -->
      <div class="flex flex-wrap gap-2">
        <span
          v-if="stats.completed > 0"
          class="px-3 py-1 rounded-full text-xs font-medium bg-green-600/80 text-green-100"
        >
          Completed {{ stats.completed }}
        </span>
        <span
          v-if="stats.in_progress > 0"
          class="px-3 py-1 rounded-full text-xs font-medium bg-blue-600/80 text-blue-100 animate-pulse"
        >
          In Progress {{ stats.in_progress }}
        </span>
        <span
          v-if="stats.pending > 0"
          class="px-3 py-1 rounded-full text-xs font-medium bg-slate-600 text-slate-300"
        >
          Pending {{ stats.pending }}
        </span>
        <span
          v-if="stats.failed > 0"
          class="px-3 py-1 rounded-full text-xs font-medium bg-red-600/80 text-red-100"
        >
          Failed {{ stats.failed }}
        </span>
        <span
          v-if="stats.skipped > 0"
          class="px-3 py-1 rounded-full text-xs font-medium bg-yellow-600/80 text-yellow-100"
        >
          Skipped {{ stats.skipped }}
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { PlanStats } from '../composables/usePlanStore';

defineProps<{
  stats: PlanStats;
  eta: string | null;
  speed: number;
  elapsedSeconds: number;
  progressPercent: number;
  phase: string;
  rateLimitInfo?: { active: boolean; remainingSeconds: number } | null;
}>();

function formatElapsed(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`;
  if (seconds < 3600) {
    const m = Math.floor(seconds / 60);
    const s = Math.round(seconds % 60);
    return `${m}m ${s}s`;
  }
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.round(seconds % 60);
  return `${h}h ${m}m ${s}s`;
}

function formatRemainingTime(seconds: number): string {
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
</script>
