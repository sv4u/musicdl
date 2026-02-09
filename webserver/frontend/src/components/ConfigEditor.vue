<template>
  <div class="space-y-4">
    <div class="flex gap-2">
      <button
        @click="loadConfig"
        class="bg-blue-600 hover:bg-blue-700 text-white font-semibold py-2 px-4 rounded transition-colors"
      >
        Load
      </button>
      <button
        @click="saveConfig"
        :disabled="!configModified"
        class="bg-green-600 hover:bg-green-700 disabled:bg-green-400 text-white font-semibold py-2 px-4 rounded transition-colors"
      >
        Save
      </button>
    </div>

    <textarea
      v-model="configContent"
      @change="configModified = true"
      class="w-full h-96 bg-slate-700 text-slate-100 border border-slate-600 rounded p-4 font-mono text-sm"
      placeholder="Config file content will appear here..."
    ></textarea>

    <div v-if="message" :class="['p-4 rounded', messageType === 'error' ? 'bg-red-900 text-red-200' : 'bg-green-900 text-green-200']">
      {{ message }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import axios from 'axios';

const configContent = ref('');
const configModified = ref(false);
const message = ref('');
const messageType = ref<'success' | 'error'>('success');

onMounted(() => {
  loadConfig();
});

async function loadConfig() {
  try {
    const response = await axios.get('/api/config');
    configContent.value = response.data.config;
    configModified.value = false;
    message.value = 'Config loaded successfully';
    messageType.value = 'success';
    setTimeout(() => (message.value = ''), 3000);
  } catch (error) {
    message.value = 'Failed to load config';
    messageType.value = 'error';
  }
}

async function saveConfig() {
  try {
    await axios.post('/api/config', { config: configContent.value });
    configModified.value = false;
    message.value = 'Config saved successfully';
    messageType.value = 'success';
    setTimeout(() => (message.value = ''), 3000);
  } catch (error) {
    message.value = 'Failed to save config';
    messageType.value = 'error';
  }
}
</script>
