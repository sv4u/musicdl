<template>
  <div class="bg-slate-700 rounded-lg p-6 border border-slate-600">
    <!-- Filter bar -->
    <div class="flex gap-4 mb-4 flex-wrap">
      <select
        v-model="statusFilter"
        class="bg-slate-800 text-slate-300 border border-slate-600 rounded px-3 py-2 text-sm"
      >
        <option value="all">All</option>
        <option value="failed">Failed</option>
        <option value="in_progress">In Progress</option>
        <option value="completed">Completed</option>
        <option value="pending">Pending</option>
        <option value="skipped">Skipped</option>
      </select>
      <input
        v-model="searchQuery"
        type="text"
        placeholder="Search by name or artist..."
        class="flex-1 min-w-[200px] bg-slate-800 text-slate-300 border border-slate-600 rounded px-3 py-2 text-sm"
      />
    </div>

    <!-- Tree view -->
    <div class="space-y-1">
      <PlanTreeNode
        v-for="node in filteredRootNodes"
        :key="node.item_id"
        :node="node"
        :get-children="getChildren"
        :status-filter="statusFilter"
        :search-query="searchQueryLower"
        :expanded-set="expandedIds"
        :has-failed-descendant="hasFailedDescendant"
        :has-matching-descendant="hasMatchingDescendant"
        @toggle="toggleExpand"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { usePlanStore } from '../composables/usePlanStore';
import type { PlanItemSnapshot } from '../composables/usePlanStore';
import PlanTreeNode from './PlanTreeNode.vue';

const props = defineProps<{
  rootNodes: PlanItemSnapshot[];
}>();

const { getChildren } = usePlanStore();

const statusFilter = ref('all');
const searchQuery = ref('');
const expandedIds = ref<Set<string>>(new Set());

const searchQueryLower = computed(() => searchQuery.value.toLowerCase().trim());

const containerTypes = new Set(['playlist', 'album', 'artist', 'm3u']);

function getArtist(item: PlanItemSnapshot): string {
  const meta = item.metadata;
  if (!meta) return '';
  const a = meta.artist ?? meta.artists;
  if (typeof a === 'string') return a;
  if (Array.isArray(a)) return a.join(', ');
  return '';
}

function matchesSearch(item: PlanItemSnapshot, query: string): boolean {
  if (!query) return true;
  const name = item.name.toLowerCase();
  const artist = getArtist(item).toLowerCase();
  return name.includes(query) || artist.includes(query);
}

function matchesStatus(item: PlanItemSnapshot, filter: string): boolean {
  if (filter === 'all') return true;
  return item.status === filter;
}

function hasFailedDescendant(parentId: string): boolean {
  const children = getChildren(parentId);
  for (const c of children) {
    if (c.status === 'failed') return true;
    if (containerTypes.has(c.item_type)) {
      if (hasFailedDescendant(c.item_id)) return true;
    }
  }
  return false;
}

function nodeMatchesFilter(node: PlanItemSnapshot): boolean {
  const isContainer = containerTypes.has(node.item_type);
  if (isContainer) {
    return hasMatchingDescendant(node.item_id);
  }
  return matchesStatus(node, statusFilter.value) && matchesSearch(node, searchQueryLower.value);
}

function hasMatchingDescendant(parentId: string): boolean {
  const children = getChildren(parentId);
  for (const c of children) {
    if (containerTypes.has(c.item_type)) {
      if (hasMatchingDescendant(c.item_id)) return true;
    } else {
      if (matchesStatus(c, statusFilter.value) && matchesSearch(c, searchQueryLower.value)) {
        return true;
      }
    }
  }
  return false;
}

const filteredRootNodes = computed(() => {
  return props.rootNodes.filter((n) => nodeMatchesFilter(n));
});

function toggleExpand(nodeId: string) {
  const next = new Set(expandedIds.value);
  if (next.has(nodeId)) {
    next.delete(nodeId);
  } else {
    next.add(nodeId);
  }
  expandedIds.value = next;
}

onMounted(() => {
  const toExpand = new Set<string>();
  function maybeExpand(nodes: PlanItemSnapshot[]) {
    for (const n of nodes) {
      if (containerTypes.has(n.item_type) && hasFailedDescendant(n.item_id)) {
        toExpand.add(n.item_id);
        maybeExpand(getChildren(n.item_id));
      }
    }
  }
  maybeExpand(props.rootNodes);
  if (toExpand.size > 0) {
    expandedIds.value = new Set([...expandedIds.value, ...toExpand]);
  }
});
</script>
