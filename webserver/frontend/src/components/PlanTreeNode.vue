<template>
  <div class="plan-tree-node">
    <!-- Container node -->
    <div
      v-if="isContainer"
      class="border border-slate-600 rounded p-3 mb-1 bg-slate-800"
    >
      <button
        class="w-full flex items-center gap-2 text-left"
        @click="$emit('toggle', node.item_id)"
      >
        <span class="text-slate-400 text-sm">
          {{ isExpanded ? '&#9660;' : '&#9658;' }}
        </span>
        <span class="text-slate-400 text-sm">{{ typeIcon }}</span>
        <span class="font-medium text-white flex-1">{{ node.name }}</span>
        <span class="text-slate-400 text-sm">{{ completedCount }}/{{ totalCount }}</span>
      </button>
      <div v-if="isExpanded" class="mt-2 ml-4 w-48 h-1.5 bg-slate-600 rounded-full overflow-hidden">
        <div
          class="h-full bg-blue-600 rounded-full transition-all"
          :style="{ width: totalCount > 0 ? `${(completedCount / totalCount) * 100}%` : '0%' }"
        />
      </div>
      <div v-if="isExpanded" class="mt-2 space-y-1">
        <PlanTreeNode
          v-for="child in filteredChildren"
          :key="child.item_id"
          :node="child"
          :get-children="getChildren"
          :status-filter="statusFilter"
          :search-query="searchQuery"
          :expanded-set="expandedSet"
          :has-failed-descendant="hasFailedDescendant"
          :has-matching-descendant="hasMatchingDescendant"
          @toggle="$emit('toggle', $event)"
        />
      </div>
    </div>

    <!-- Track node -->
    <div
      v-else
      class="flex items-center gap-2 py-2 px-3 rounded border border-slate-600/50 bg-slate-800/50 mb-1"
      :class="{ 'hidden': !trackVisible }"
    >
      <span :class="statusIconClass" class="flex-shrink-0">
        <span v-if="node.status === 'pending'" class="text-slate-400">&#9675;</span>
        <span v-else-if="node.status === 'in_progress'" class="text-blue-400 animate-spin inline-block">&#8635;</span>
        <span v-else-if="node.status === 'completed'" class="text-green-400">&#10003;</span>
        <span v-else-if="node.status === 'failed'" class="text-red-400">&#10005;</span>
        <span v-else-if="node.status === 'skipped'" class="text-yellow-400">&#8855;</span>
        <span v-else class="text-slate-400">&#9675;</span>
      </span>
      <div class="min-w-0 flex-1">
        <span class="text-white font-medium">{{ node.name }}</span>
        <span v-if="artist" class="text-slate-400 text-sm ml-2">{{ artist }}</span>
        <p v-if="node.status === 'failed' && node.error" class="text-red-400 text-sm mt-0.5">
          {{ node.error }}
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { PlanItemSnapshot } from '../composables/usePlanStore';

const props = defineProps<{
  node: PlanItemSnapshot;
  getChildren: (parentId: string) => PlanItemSnapshot[];
  statusFilter: string;
  searchQuery: string;
  expandedSet: Set<string>;
  hasFailedDescendant: (parentId: string) => boolean;
  hasMatchingDescendant: (parentId: string) => boolean;
}>();

defineEmits<{
  toggle: [nodeId: string];
}>();

const containerTypes = new Set(['playlist', 'album', 'artist', 'm3u']);

const isContainer = computed(() => containerTypes.has(props.node.item_type));

const isExpanded = computed(() => props.expandedSet.has(props.node.item_id));

const children = computed(() => {
  if (!isContainer.value) return [];
  return props.getChildren(props.node.item_id);
});

const filteredChildren = computed(() => {
  const q = props.searchQuery.toLowerCase().trim();
  return children.value.filter((c) => {
    if (containerTypes.has(c.item_type)) {
      return props.hasMatchingDescendant(c.item_id);
    }
    return matchesStatus(c, props.statusFilter) && matchesSearch(c, q);
  });
});

const completedCount = computed(() => {
  return children.value.filter(
    (c) => c.item_type === 'track' && (c.status === 'completed' || c.status === 'skipped' || c.status === 'failed')
  ).length;
});

const totalCount = computed(() => {
  return children.value.filter((c) => c.item_type === 'track').length;
});

function getArtist(item: PlanItemSnapshot): string {
  const meta = item.metadata;
  if (!meta) return '';
  const a = meta.artist ?? meta.artists;
  if (typeof a === 'string') return a;
  if (Array.isArray(a)) return a.join(', ');
  return '';
}

const artist = computed(() => getArtist(props.node));

function matchesSearch(item: PlanItemSnapshot, query: string): boolean {
  if (!query) return true;
  const name = item.name.toLowerCase();
  const a = getArtist(item).toLowerCase();
  return name.includes(query) || a.includes(query);
}

function matchesStatus(item: PlanItemSnapshot, filter: string): boolean {
  if (filter === 'all') return true;
  return item.status === filter;
}

const trackVisible = computed(() => {
  if (isContainer.value) return true;
  return (
    matchesStatus(props.node, props.statusFilter) &&
    matchesSearch(props.node, props.searchQuery)
  );
});

const typeIcon = computed(() => {
  const t = props.node.item_type;
  if (t === 'playlist') return '[P]';
  if (t === 'album') return '[A]';
  if (t === 'artist') return '[AR]';
  if (t === 'm3u') return '[M]';
  return '';
});

const statusIconClass = computed(() => 'text-base');
</script>
