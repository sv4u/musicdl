<template>
  <div class="bg-slate-800 rounded-lg p-3 border border-slate-600">
    <div class="text-slate-500 text-xs uppercase tracking-wider">{{ label }}</div>
    <div :class="['text-lg font-bold mt-1', textColorClass]">{{ value }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

interface Props {
  label: string;
  value: number | string;
  color?: string;
}

const props = withDefaults(defineProps<Props>(), {
  color: 'white',
});

const textColorClass = computed(() => {
  const map: Record<string, string> = {
    green: 'text-green-400',
    red: 'text-red-400',
    yellow: 'text-yellow-400',
    blue: 'text-blue-400',
    purple: 'text-purple-400',
    orange: 'text-orange-400',
    indigo: 'text-indigo-400',
    white: 'text-white',
  };
  return map[props.color] || 'text-white';
});
</script>
