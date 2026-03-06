<template>
  <div class="bg-slate-700 rounded-lg p-6 border border-slate-600">
    <div class="flex items-center gap-4">
      <svg
        class="w-8 h-8 text-blue-400 animate-spin flex-shrink-0"
        xmlns="http://www.w3.org/2000/svg"
        fill="none"
        viewBox="0 0 24 24"
        aria-hidden="true"
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
      <div>
        <p class="text-white font-medium">{{ message || 'Generating plan...' }}</p>
        <p v-if="itemsFound > 0" class="text-slate-400 text-sm mt-1">
          {{ itemsFound }} items found
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  message: string;
  itemsFound: number;
}>();
</script>
