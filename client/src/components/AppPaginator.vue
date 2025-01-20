<template>
  <div class="d-flex flex-wrap justify-center">
    <v-select
      v-model="pagination.pageSize"
      :items="[10, 25, 50, 100, 250, 500, 1000]"
      label="Page size"
      style="width: min-content;"
      class="w-min"
    />
    <v-pagination
      v-if="pagination"
      v-model="pagination.page"
      :length="pagination.totalPages"
      :total-visible="$vuetify.display.smAndDown ? 4 : 12"
      prev-icon="fas fa-chevron-left"
      next-icon="fas fa-chevron-right"
    />
  </div>
</template>

<script lang="ts">
import { defineComponent } from "vue";

export default defineComponent({
  props: {
    pagination: {
      type: Object as () => import("@/api/common").PageMeta,
      required: true,
    },
  },
});
</script>