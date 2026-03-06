<template>
  <div class="bg-slate-700 rounded-lg p-6 border border-slate-600">
    <h3 class="text-white font-semibold mb-4">Currently Downloading</h3>
    <ul class="space-y-3">
      <li
        v-for="item in items"
        :key="item.item_id"
        class="flex items-center gap-3 text-slate-300"
      >
        <svg
          class="w-5 h-5 text-blue-400 animate-spin flex-shrink-0"
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
        <div class="min-w-0 flex-1">
          <span class="font-medium text-white">{{ item.name }}</span>
          <span
            v-if="artist(item)"
            class="text-slate-400 text-sm ml-2"
          >
            {{ artist(item) }}
          </span>
        </div>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import type { PlanItemSnapshot } from '../composables/usePlanStore';

const props = defineProps<{
  items: PlanItemSnapshot[];
}>();

function artist(item: PlanItemSnapshot): string {
  const meta = item.metadata;
  if (!meta) return '';
  const a = meta.artist ?? meta.artists;
  if (typeof a === 'string') return a;
  if (Array.isArray(a)) return a.join(', ');
  return '';
}
</script>
