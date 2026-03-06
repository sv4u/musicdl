<template>
  <div class="bg-red-950/30 rounded-lg p-6 border border-red-800">
    <div class="flex items-center justify-between mb-4">
      <h3 class="text-red-200 font-semibold">Failed Items ({{ items.length }})</h3>
      <div class="flex gap-2">
        <button
          @click="copyAllErrors"
          class="px-3 py-1.5 bg-slate-600 hover:bg-slate-500 text-white text-sm font-medium rounded transition-colors"
        >
          Copy All Errors
        </button>
        <button
          @click="exportCsv"
          class="px-3 py-1.5 bg-slate-600 hover:bg-slate-500 text-white text-sm font-medium rounded transition-colors"
        >
          Export CSV
        </button>
      </div>
    </div>

    <ul class="space-y-4">
      <li
        v-for="item in items"
        :key="item.item_id"
        class="border border-red-800/50 rounded p-4 bg-red-950/20"
      >
        <div class="flex items-start gap-3">
          <span class="text-red-400 text-lg flex-shrink-0" aria-hidden="true">&#10005;</span>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="font-medium text-red-100">{{ item.name }}</span>
              <span v-if="getArtist(item)" class="text-red-300/80 text-sm">
                {{ getArtist(item) }}
              </span>
            </div>
            <p v-if="item.error" class="text-red-400 text-sm mt-1">{{ item.error }}</p>
            <div class="mt-2 flex gap-2">
              <button
                @click="toggleRaw(item.item_id)"
                class="text-xs text-slate-400 hover:text-slate-300 underline"
              >
                {{ expandedRaw.has(item.item_id) ? 'Hide raw output' : 'Show raw output' }}
              </button>
              <button
                @click="copyItem(item)"
                class="text-xs text-slate-400 hover:text-slate-300 underline"
              >
                Copy
              </button>
            </div>
            <pre
              v-if="expandedRaw.has(item.item_id) && item.raw_output"
              class="mt-2 p-3 bg-slate-950 rounded text-slate-300 text-xs font-mono overflow-x-auto whitespace-pre-wrap"
            >{{ item.raw_output }}</pre>
          </div>
        </div>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import type { PlanItemSnapshot } from '../composables/usePlanStore';

const props = defineProps<{
  items: PlanItemSnapshot[];
}>();

const expandedRaw = ref<Set<string>>(new Set());

function getArtist(item: PlanItemSnapshot): string {
  const meta = item.metadata;
  if (!meta) return '';
  const a = meta.artist ?? meta.artists;
  if (typeof a === 'string') return a;
  if (Array.isArray(a)) return a.join(', ');
  return '';
}

function toggleRaw(itemId: string) {
  const next = new Set(expandedRaw.value);
  if (next.has(itemId)) {
    next.delete(itemId);
  } else {
    next.add(itemId);
  }
  expandedRaw.value = next;
}

function buildItemText(item: PlanItemSnapshot): string {
  const lines: string[] = [];
  lines.push(`Track: ${item.name}`);
  const artist = getArtist(item);
  if (artist) lines.push(`Artist: ${artist}`);
  if (item.error) lines.push(`Error: ${item.error}`);
  if (item.raw_output) lines.push(`Raw output:\n${item.raw_output}`);
  return lines.join('\n');
}

async function copyAllErrors() {
  const block = props.items.map((item) => buildItemText(item)).join('\n\n---\n\n');
  await navigator.clipboard.writeText(block);
}

function copyItem(item: PlanItemSnapshot) {
  navigator.clipboard.writeText(buildItemText(item));
}

function exportCsv() {
  const headers = ['Track Name', 'Artist', 'Error', 'Raw Output', 'Spotify URL', 'YouTube URL'];
  const rows = props.items.map((item) => {
    const artist = getArtist(item);
    return [
      escapeCsvField(item.name),
      escapeCsvField(artist),
      escapeCsvField(item.error ?? ''),
      escapeCsvField(item.raw_output ?? ''),
      escapeCsvField(item.spotify_url ?? ''),
      escapeCsvField(item.youtube_url ?? ''),
    ];
  });
  const csv = [headers.join(','), ...rows.map((r) => r.join(','))].join('\n');
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `failed-items-${new Date().toISOString().slice(0, 10)}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

function escapeCsvField(value: string): string {
  if (!value.includes(',') && !value.includes('"') && !value.includes('\n')) {
    return value;
  }
  return `"${value.replace(/"/g, '""')}"`;
}
</script>
