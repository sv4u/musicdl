<template>
  <div class="bg-slate-800 rounded-lg p-6 border border-slate-700">
    <div class="flex items-center justify-between gap-4 mb-4">
      <h2 class="text-lg font-semibold text-white">YouTube cookies</h2>
      <button
        type="button"
        class="text-xs px-3 py-1.5 rounded-md bg-slate-700 text-slate-200 hover:bg-slate-600 border border-slate-600 transition-colors"
        :disabled="loading"
        @click="fetchStatus(true)"
      >
        {{ loading ? 'Checking…' : 'Re-check probe' }}
      </button>
    </div>

    <p v-if="error" class="text-red-400 text-sm mb-3">{{ error }}</p>
    <p v-else-if="!data && !loading" class="text-slate-500 text-sm mb-3">No status yet.</p>

    <template v-else-if="data">
      <div class="flex items-center gap-2 mb-4 text-slate-300 text-sm">
        <span class="text-slate-500">Source:</span>
        <span class="font-medium text-white capitalize">{{ data.source }}</span>
        <span v-if="data.cookiesPath" class="truncate text-slate-400" :title="data.cookiesPath">{{ data.cookiesPath }}</span>
        <span v-if="data.cookiesFromBrowser" class="text-slate-400">browser: {{ data.cookiesFromBrowser }}</span>
      </div>

      <div class="space-y-4">
        <div>
          <div class="flex items-center gap-2 mb-1">
            <div
              :class="[
                'w-3 h-3 rounded-full flex-shrink-0',
                tierAOk === true ? 'bg-green-400' : tierAOk === false ? 'bg-amber-400' : 'bg-slate-500',
              ]"
              role="status"
              :aria-label="'Tier A: file and jar checks ' + (tierAOk === true ? 'passed' : tierAOk === false ? 'issues' : 'unknown')"
            />
            <span class="text-slate-200 font-medium text-sm">Tier A — file &amp; jar</span>
            <InfoTooltip text="Checks that the configured path exists, is readable, is non-empty, and contains YouTube/Google cookie rows (Netscape format). For browser mode, only that cookies_from_browser is set." />
          </div>
          <ul v-if="tierAIssues.length" class="text-amber-200/90 text-xs mt-1 list-disc list-inside space-y-0.5">
            <li v-for="(msg, i) in tierAIssues" :key="i">{{ msg }}</li>
          </ul>
          <ul v-else-if="data.source === 'browser'" class="text-slate-500 text-xs mt-1">Browser cookie mode — no file checks.</ul>
          <ul v-else-if="data.source === 'none'" class="text-slate-500 text-xs mt-1">No cookie file or browser setting in config.</ul>
          <ul v-else class="text-green-400/90 text-xs mt-1">Cookie file looks present and non-empty with YouTube/Google rows.</ul>
        </div>

        <div>
          <div class="flex items-center gap-2 mb-1">
            <div
              :class="[
                'w-3 h-3 rounded-full flex-shrink-0',
                tierBDotClass,
              ]"
              role="status"
              :aria-label="'Tier B: yt-dlp probe ' + (data.tierB?.skipped ? 'skipped' : data.tierB?.valid ? 'passed' : 'failed')"
            />
            <span class="text-slate-200 font-medium text-sm">Tier B — yt-dlp probe</span>
            <InfoTooltip text="Runs yt-dlp --skip-download on a probe URL with your cookie and JS settings (same as downloads). Cached on the server for 5 minutes; use Re-check probe to bypass." />
          </div>
          <p v-if="data.tierB?.skipped" class="text-slate-400 text-xs mt-1">{{ data.tierB?.message }}</p>
          <template v-else>
            <p class="text-slate-500 text-xs mt-1 break-all">
              <span class="text-slate-500">URL:</span> {{ data.tierB?.probeUrl }}
              <span v-if="data.tierB?.checkedAtUnix" class="ml-2 text-slate-600">
                (checked {{ formatCheckedAt(data.tierB.checkedAtUnix) }})
              </span>
            </p>
            <p
              v-if="data.tierB?.message && !data.tierB?.valid"
              class="text-red-300/90 text-xs mt-2 whitespace-pre-wrap break-words max-h-32 overflow-y-auto"
            >
              {{ data.tierB?.message }}
            </p>
            <p v-else-if="data.tierB?.valid" class="text-green-400/90 text-xs mt-2">Probe succeeded — cookies and yt-dlp can extract the probe video.</p>
          </template>
        </div>
      </div>

      <p v-if="data.configError" class="text-amber-300 text-xs mt-4 border-t border-slate-700 pt-3">
        Config error: {{ data.configError }}
      </p>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import axios from 'axios';
import InfoTooltip from './InfoTooltip.vue';

interface TierA {
  ready?: boolean;
  fileExists?: boolean;
  fileReadable?: boolean;
  fileSizeBytes?: number;
  hasYoutubeOrGoogleJarLine?: boolean;
  issues?: string[];
}

interface TierB {
  skipped?: boolean;
  valid?: boolean | null;
  message?: string;
  probeUrl?: string;
  checkedAtUnix?: number;
}

interface CookiesStatusResponse {
  source: string;
  cookiesPath?: string;
  cookiesPathRaw?: string;
  cookiesFromBrowser?: string;
  configError?: string;
  tierA?: TierA;
  tierB?: TierB;
  error?: string;
}

const data = ref<CookiesStatusResponse | null>(null);
const error = ref('');
const loading = ref(false);
let pollTimer: ReturnType<typeof setTimeout> | null = null;
let unmounted = false;

const tierAIssues = computed(() => data.value?.tierA?.issues ?? []);

const tierAOk = computed(() => {
  const d = data.value;
  if (!d?.tierA) return null;
  if (d.source === 'browser') return !!d.tierA.ready;
  if (d.source === 'file') return !!d.tierA.ready;
  if (d.source === 'none') return false;
  return null;
});

const tierBDotClass = computed(() => {
  const b = data.value?.tierB;
  if (!b || b.skipped) return 'bg-slate-500';
  if (b.valid === true) return 'bg-green-400';
  if (b.valid === false) return 'bg-red-400';
  return 'bg-slate-500';
});

function formatCheckedAt(unix: number): string {
  try {
    return new Date(unix * 1000).toLocaleString();
  } catch {
    return '';
  }
}

async function fetchStatus(force: boolean) {
  if (pollTimer !== null) {
    clearTimeout(pollTimer);
    pollTimer = null;
  }
  loading.value = true;
  error.value = '';
  try {
    const url = force ? '/api/cookies-status?force=1' : '/api/cookies-status';
    const res = await axios.get<CookiesStatusResponse>(url);
    data.value = res.data;
  } catch (e) {
    if (axios.isAxiosError(e) && e.response?.status === 404) {
      error.value = 'No config.yaml — cookie status unavailable.';
      data.value = null;
    } else {
      error.value = 'Could not load cookie status.';
      data.value = null;
    }
  } finally {
    loading.value = false;
  }
  if (!unmounted) {
    pollTimer = setTimeout(() => fetchStatus(false), 45000);
  }
}

onMounted(() => {
  fetchStatus(false);
});

onUnmounted(() => {
  unmounted = true;
  if (pollTimer !== null) clearTimeout(pollTimer);
});
</script>
