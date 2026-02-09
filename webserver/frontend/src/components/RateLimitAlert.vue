<template>
  <div class="bg-red-950 border border-red-700 rounded-lg p-6">
    <div class="flex items-start gap-4">
      <div class="flex-shrink-0">
        <svg class="w-6 h-6 text-red-400" fill="currentColor" viewBox="0 0 20 20">
          <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
        </svg>
      </div>
      <div class="flex-1">
        <h2 class="text-lg font-semibold text-red-200">Spotify Rate Limit Active</h2>
        <p class="text-red-300 mt-2">
          Rate limit detected. Downloads are paused.
        </p>
        <div class="mt-4">
          <p class="text-2xl font-bold text-red-400 font-mono">{{ remainingTime }}</p>
          <p class="text-sm text-red-300">seconds remaining</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue';

interface RateLimitInfo {
  active: boolean;
  retryAfterSeconds: number;
  retryAfterTimestamp: number;
  detectedAt: number;
  remainingSeconds: number;
}

interface Props {
  info: RateLimitInfo;
}

defineProps<Props>();

const remainingTime = ref(0);

const props = defineProps<Props>();

watch(
  () => props.info,
  (newInfo) => {
    if (newInfo.active) {
      updateCountdown();
    }
  },
  { immediate: true }
);

function updateCountdown() {
  const now = Math.floor(Date.now() / 1000);
  remainingTime.value = Math.max(0, props.info.retryAfterTimestamp - now);

  if (remainingTime.value > 0) {
    setTimeout(updateCountdown, 1000);
  }
}
</script>
