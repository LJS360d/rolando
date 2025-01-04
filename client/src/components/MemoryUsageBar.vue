<template>
  <template v-if="isDataAvailable">
    <div style="height: 20px;" class="position-relative d-flex justify-end pa-0 ma-0 w-full">
      <span class="text-caption">
        {{ formatBytes(max) }}
      </span>
      <span class="position-absolute bottom-0 w-max-content text-caption"
        :style="{ left: ((current / max) * (100) - 1.5) + '%' }">
        {{ formatBytes(current) }}
      </span>
    </div>
    <v-progress-linear :value="max" height="20" color="grey lighten-2" class="rounded-lg overflow-visible" :max="max">
      <div class="mem-breakpoint bg-light-blue" :style="{ left: (current / max) * (100) + '%' }">
      </div>
      <!-- Peak line -->
      <v-tooltip v-slot:activator="{ props }" :text="formatBytes(peak)" location="top center">
        <div v-bind="props" class="mem-breakpoint bg-red" :style="{ left: (peak / max) * (100) + '%' }">
        </div>
      </v-tooltip>
      <!-- Memory blocks -->
      <div v-for="(block, index) in blocks" :v-if="!!block" :key="index" class="memory-block" :style="{
        width: ((block) / max) * (100) + '%',
        backgroundColor: getBlockColor(index),
      }">
      </div>
    </v-progress-linear>
    <div style="height: 20px;" class="position-relative d-flex justify-end pa-0 ma-0 w-full">
      <!-- Peak label -->
      <span class="position-absolute w-max-content text-caption" :style="{ left: ((peak / max) * (100) - 1.5) + '%' }">
        {{ formatBytes(peak) }}
      </span>
    </div>
  </template>
  <!-- Skeleton loader when data is unavailable -->
  <v-progress-linear v-else indeterminate height="20" color="grey lighten-3" class="rounded-lg" />
</template>

<script lang="ts">
import { formatBytes } from '@/utils/format';
import { defineComponent, type PropType } from 'vue';

export default defineComponent({
  name: 'MemoryUsageBar',
  props: {
    max: {
      type: Number,
      required: true,
    },
    current: {
      type: Number,
      required: true,
    },
    peak: {
      type: Number,
      required: true,
    },
    blocks: {
      type: Array as PropType<number[]>,
      required: false,
    },
  },
  data() {
    return {
      max: this.max,
      peak: this.peak,
      current: this.current
    };
  },
  computed: {
    isDataAvailable(): boolean {
      return !!this.max && !!this.peak && !!this.blocks;
    },
  },
  methods: {
    formatBytes,
    getBlockColor(index: number): string {
      const colors = ['#4caf5069', '#ffeb3b69', '#f4433669', '#2196f369', '#9c27b069'];
      return colors[index % colors.length];
    },
  },
});
</script>

<style>
.memory-block {
  height: 100%;
  transition: all 0.3s ease;
}

.v-progress-linear__content {
  justify-content: flex-start !important;
}

.mem-breakpoint {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 3px;
}

.w-max-content {
  width: max-content;
}
</style>
