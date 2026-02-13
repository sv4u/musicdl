<template>
  <div class="space-y-6">
    <div class="flex gap-4">
      <button
        @click="generatePlan"
        :disabled="isRunning"
        class="flex-1 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 text-white font-semibold py-3 rounded-lg transition-colors"
      >
        {{ isRunning && operationType === 'plan' ? 'Generating Plan...' : 'Generate Plan' }}
      </button>
      <button
        @click="runDownload(false)"
        :disabled="isRunning"
        class="flex-1 bg-green-600 hover:bg-green-700 disabled:bg-green-400 text-white font-semibold py-3 rounded-lg transition-colors"
      >
        {{ isRunning && operationType === 'download' ? 'Downloading...' : 'Run Download' }}
      </button>
      <button
        v-if="hasResumeData"
        @click="runDownload(true)"
        :disabled="isRunning"
        class="bg-yellow-600 hover:bg-yellow-700 disabled:bg-yellow-400 text-white font-semibold py-3 px-4 rounded-lg transition-colors"
        title="Resume interrupted download"
      >
        Resume
      </button>
    </div>

    <!-- Progress Indicator -->
    <div v-if="isRunning" class="bg-slate-700 rounded-lg p-6 border border-slate-600">
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-white font-semibold">{{ operationType === 'plan' ? 'Generating Plan' : 'Downloading' }}</h3>
        <span class="text-slate-400 text-sm">{{ progress }}/{{ total }}</span>
      </div>
      <div class="w-full bg-slate-600 rounded-full h-2">
        <div
          class="bg-blue-600 h-2 rounded-full transition-all"
          :style="{ width: total > 0 ? `${(progress / total) * 100}%` : '0%' }"
        ></div>
      </div>
    </div>

    <!-- Error Display -->
    <div v-if="errorDetail" class="bg-red-950/50 border border-red-700 rounded-lg p-6">
      <div class="flex items-start gap-3">
        <svg class="w-5 h-5 text-red-400 mt-0.5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
          <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
        </svg>
        <div>
          <h4 class="text-red-200 font-semibold">{{ errorDetail.message }}</h4>
          <p class="text-red-300 text-sm mt-1">{{ errorDetail.explanation }}</p>
          <p class="text-red-400 text-sm mt-2">{{ errorDetail.suggestion }}</p>
          <span
            v-if="errorDetail.retryable"
            class="inline-block mt-2 text-xs bg-yellow-600/30 text-yellow-300 px-2 py-1 rounded"
          >
            Retryable
          </span>
          <span
            v-else
            class="inline-block mt-2 text-xs bg-red-600/30 text-red-300 px-2 py-1 rounded"
          >
            Not retryable
          </span>
        </div>
      </div>
    </div>

    <!-- Status Display -->
    <div class="bg-slate-700 rounded-lg p-4 border border-slate-600">
      <h3 class="text-white font-semibold mb-3">Status</h3>
      <p class="text-slate-300">
        {{ statusMessage }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import axios from 'axios';

interface ErrorDetail {
  code: string;
  message: string;
  explanation: string;
  suggestion: string;
  retryable: boolean;
  timestamp: number;
}

const isRunning = ref(false);
const operationType = ref('');
const progress = ref(0);
const total = ref(0);
const statusMessage = ref('Ready to start');
const errorDetail = ref<ErrorDetail | null>(null);
const hasResumeData = ref(false);

onMounted(() => {
  pollStatus();
  checkResumeState();
});

async function generatePlan() {
  try {
    errorDetail.value = null;
    await axios.post('/api/download/plan', { configPath: '/download/config.yaml' });
    isRunning.value = true;
    operationType.value = 'plan';
    pollStatus();
  } catch (error) {
    if (axios.isAxiosError(error) && error.response?.status === 503) {
      statusMessage.value = 'Circuit breaker is open - too many consecutive failures. Check the Statistics tab for recovery options.';
    } else {
      statusMessage.value = 'Error: Failed to start plan generation';
    }
  }
}

async function runDownload(resume: boolean) {
  try {
    errorDetail.value = null;
    await axios.post('/api/download/run', {
      configPath: '/download/config.yaml',
      resume: resume ? 'true' : 'false',
    });
    isRunning.value = true;
    operationType.value = 'download';
    statusMessage.value = resume ? 'Resuming download...' : 'Starting download...';
    pollStatus();
  } catch (error) {
    if (axios.isAxiosError(error) && error.response?.status === 503) {
      statusMessage.value = 'Circuit breaker is open - too many consecutive failures. Check the Statistics tab for recovery options.';
    } else {
      statusMessage.value = 'Error: Failed to start download';
    }
  }
}

async function pollStatus() {
  try {
    const response = await axios.get('/api/download/status');
    isRunning.value = response.data.isRunning;
    operationType.value = response.data.operationType || '';
    progress.value = response.data.progress || 0;
    total.value = response.data.total || 0;

    // Handle classified error details.
    // Clear stale errors when the backend no longer reports one, so the
    // UI doesn't keep showing an old error after the state has recovered.
    if (response.data.error) {
      errorDetail.value = response.data.error;
    } else {
      errorDetail.value = null;
    }

    if (isRunning.value) {
      statusMessage.value = `${operationType.value === 'plan' ? 'Generating plan' : 'Downloading'}... ${progress.value}/${total.value}`;
      setTimeout(pollStatus, 1000);
    } else {
      if (!errorDetail.value) {
        statusMessage.value = 'Idle';
      }
      // Refresh resume state every idle poll so the Resume button appears
      // promptly after a partial failure — not only on page load.
      await checkResumeState();
      setTimeout(pollStatus, 5000);
    }
  } catch {
    statusMessage.value = 'Error connecting to API';
    setTimeout(pollStatus, 5000);
  }
}

async function checkResumeState() {
  try {
    const response = await axios.get('/api/recovery/status');
    hasResumeData.value = response.data.resume?.hasResumeData || false;
  } catch { /* ignore */ }
}
</script>
