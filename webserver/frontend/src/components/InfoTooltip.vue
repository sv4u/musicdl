<template>
  <span class="info-tooltip-wrapper">
    <button
      type="button"
      class="info-tooltip-trigger"
      :aria-label="text"
      @mouseenter="visible = true"
      @mouseleave="visible = false"
      @focus="visible = true"
      @blur="visible = false"
    >
      ?
    </button>
    <span v-show="visible" class="info-tooltip-content" role="tooltip">
      {{ text }}
    </span>
  </span>
</template>

<script setup lang="ts">
import { ref } from "vue";

defineProps<{
  text: string;
}>();

const visible = ref(false);
</script>

<style scoped>
.info-tooltip-wrapper {
  position: relative;
  display: inline-flex;
  align-items: center;
  margin-left: 4px;
}

.info-tooltip-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border-radius: 9999px;
  border: 1px solid #64748b;
  background: transparent;
  color: #64748b;
  font-size: 10px;
  font-weight: 600;
  line-height: 1;
  cursor: help;
  padding: 0;
  transition:
    border-color 0.15s,
    color 0.15s;
}

.info-tooltip-trigger:hover,
.info-tooltip-trigger:focus-visible {
  border-color: #94a3b8;
  color: #94a3b8;
  outline: none;
}

.info-tooltip-content {
  position: absolute;
  bottom: calc(100% + 8px);
  left: 50%;
  transform: translateX(-50%);
  white-space: normal;
  width: max-content;
  max-width: 240px;
  padding: 8px 12px;
  border-radius: 8px;
  background: #1e293b;
  border: 1px solid #475569;
  color: #cbd5e1;
  font-size: 12px;
  line-height: 1.4;
  z-index: 50;
  pointer-events: none;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
}

.info-tooltip-content::after {
  content: "";
  position: absolute;
  top: 100%;
  left: 50%;
  transform: translateX(-50%);
  border: 5px solid transparent;
  border-top-color: #475569;
}
</style>
