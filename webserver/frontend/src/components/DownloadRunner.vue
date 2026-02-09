<template>
  <div class="space-y-6">
    <div class="flex gap-4">
      <button
        @click="generatePlan"
        :disabled="isRunning"
        class="flex-1 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 text-white font-semibold py-3 rounded-lg transition-colors"
      >
        {{ isRunning ? 'Running...' : 'Generate Plan' }}
      </button>
      <button
        @click="runDownload"
        :disabled="isRunning"
        class="flex-1 bg-green-600 hover:bg-green-700 disabled:bg-green-400 text-white font-semibold py-3 rounded-lg transition-colors"
      >
        {{ isRunning ? 'Running...' : 'Run Download' }}
      </button>
    </div>

    <!-- Progress Indicator -->
    <div v-if="isRunning" class="bg-slate-700 rounded-lg p-6 border border-slate-600">
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-white font-semibold">{{ operationType }}</h3>
        <span class="text-slate-400 text-sm">{{ progress }}/{{ total }}</span>
      </div>
      <div class="w-full bg-slate-600 rounded-full h-2">
        <div
          class="bg-blue-600 h-2 rounded-full transition-all"
          :style="{ width: total > 0 ? `${(progress / total) * 100}%` : '0%' }"
        ></div>
      </div>
    </div>

    <!-- Status Display -->
    <div class="bg-slate-700 rounded-lg p-4 border border-slate-600">
      <h3 class="text-white font-semibold mb-3">Status</h3>
      <p class="text-slate-300">
        {{ status }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import axios from 'axios';

const isRunning = ref(false);
const operationType = ref('');
const progress = ref(0);
const total = ref(0);
const status = ref('Ready to start');

onMounted(() => {
  pollStatus();
});

async function generatePlan() {
  try {
    await axios.post('/api/download/plan', { configPath: '/download/config.yaml' });
    isRunning.value = true;
    operationType.value = 'Generating Plan';
    pollStatus();
  } catch (error) {
    status.value = 'Error: Failed to start plan generation';
  }
}

async function runDownload() {
  try {
    await axios.post('/api/download/run', { configPath: '/download/config.yaml' });
    isRunning.value = true;
    operationType.value = 'Running Download';
    pollStatus();
  } catch (error) {
    status.value = 'Error: Failed to start download';
  }
}

async function pollStatus() {
  try {
    const response = await axios.get('/api/download/status');
    isRunning.value = response.data.isRunning;
    operationType.value = response.data.operationType || 'Idle';
    progress.value = response.data.progress || 0;
    total.value = response.data.total || 0;
    status.value = response.data.status || 'Idle';

    if (isRunning.value) {
      setTimeout(pollStatus, 1000);
    } else {
      status.value = 'Idle';
      setTimeout(pollStatus, 5000);
    }
  } catch {
    status.value = 'Error connecting to API';
    setTimeout(pollStatus, 5000);
  }
}
</script>
