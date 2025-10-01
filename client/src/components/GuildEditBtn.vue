<template>
  <v-btn icon="fas fa-edit" size="small" @click="open = true" />
  <v-dialog v-model="open" :max-width="425">
    <v-card :prepend-avatar="guildIconUrl(guild.id, guild.icon)">
      <template #title>
        <span class="font-weight-light">Edit Guild</span>
      </template>
      <template #subtitle>
        <span class="text-sm">{{ guild.name }} | <b>{{ guild.approximate_member_count }}</b> members</span>
      </template>
      <template #text>
        <v-col>
          <v-switch v-model="fields.pings_enabled" label="Enable pings" inset dense color="primary" />
          <v-switch v-model="fields.trained" label="Trained" inset dense color="primary" />
          <v-switch v-model="fields.premium" label="Premium" inset dense color="primary" />
          <v-text-field v-model="fields.reply_rate" type="number" label="Reply Rate" outlined dense />
          <v-text-field v-model="fields.n_gram_size" type="number" label="N Gram Size" outlined dense />
          <v-text-field v-model="fields.max_size_mb" type="number" label="Max Size (MB)" outlined dense />
        </v-col>
      </template>
      <v-card-actions>
        <v-spacer />
        <v-btn @click="open = false">
          Cancel
        </v-btn>
        <v-btn color="primary" @click="confirm">
          Confirm
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import type { ChainAnalytics } from '@/api/analytics';
import type { BotGuild } from '@/api/bot';
import { guildIconUrl } from '@/utils/format';
import { ref, type PropType } from 'vue';

export default defineComponent({
  name: 'GuildEditBtn',
  props: {
    guild: {
      type: Object as PropType<BotGuild>,
      required: true,
    },
    chain: {
      type: Object as PropType<ChainAnalytics>,
      required: true,
    }
  },
  emits: ['update:modelValue', 'confirm', 'cancel'],
  setup(props, { emit }) {
    const open = ref(false)
    const fields = ref<Partial<ChainAnalytics>>({ ...(props.chain ? props.chain : {}) });
    const confirm = async () => {
      emit('confirm', fields);
      open.value = false;
    };

    return {
      confirm,
      fields,
      open,
    }
  },
  methods: {
    guildIconUrl,
  }
})

</script>
