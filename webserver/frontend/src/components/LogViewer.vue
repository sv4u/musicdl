<template>
  <div class="space-y-4">
    <div class="flex gap-2">
      <button
        @click="loadLogs"
        class="bg-blue-600 hover:bg-blue-700 text-white font-semibold py-2 px-4 rounded transition-colors"
      >
        Refresh
      </button>
      <button
        @click="clearLogs"
        class="bg-red-600 hover:bg-red-700 text-white font-semibold py-2 px-4 rounded transition-colors"
      >
        Clear
      </button>
    </div>

    <div class="bg-slate-950 border border-slate-600 rounded p-4 h-96 overflow-y-auto font-mono text-sm">
      <div v-if="logs.length === 0" class="text-slate-500">
        No logs yet...
      </div>
      <div v-else class="text-slate-300 space-y-1">
        <div v-for="(log, index) in logs" :key="index" class="text-slate-400">
          {{ log }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import axios from 'axios';

const logs = ref<string[]>([]);

onMounted(() => {
  loadLogs();
  setInterval(loadLogs, 2000);
});

async function loadLogs() {
  try {
    const response = await axios.get('/api/logs');
    logs.value = response.data.logs || [];
  } catch {
    logs.value = ['Error loading logs'];
  }
}

function clearLogs() {
  logs.value = [];
}
</script>
