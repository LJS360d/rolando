<template>
  <v-container>
    <template v-if="!isLoadingGuild && !isErrorGuild && guild">
      <v-card
        flat
        :prepend-avatar="guildIconUrl(guild.id, guild.icon)"
      >
        <template #title>
          <span class="font-weight-light">{{ guild.name }}</span>
        </template>
        <template #subtitle>
          <span class="text-sm"><b>{{ guild.approximate_member_count }}</b> members</span>
        </template>
        <template
          v-if="!!chain"
          #text
        >
          <v-row
            justify="center"
            class="pa-3 pb-0"
          >
            <span>{{ formatBytes(chain?.bytes ?? 0) }} / {{ formatBytes(1024 ** 2 *
              (chain?.max_size_mb ?? 0)) }}</span>
          </v-row>
          <v-row
            justify="space-between"
            class="pa-3"
          >
            <v-col cols="12">
              <v-row
                v-for="(field, key) in getChainAnalytics()"
                :key="key"
                justify="space-between"
              >
                <span class="text-xs">{{ key }}</span>
                <span class="text-xs">{{ formatNumber(field) }}</span>
              </v-row>
            </v-col>
          </v-row>
        </template>
      </v-card>
    </template>
    <v-skeleton-loader
      v-else-if="!isErrorGuild"
      type="card-avatar"
    />
    <v-alert
      v-else
      type="error"
      class="text-body-2"
    >
      Oops, big error occured, please report it to the creator on <a :href="discordServerInvite">the discord</a>
    </v-alert>
    <v-divider class="my-4" />
    <template v-if="pagination.totalPages > 1">
      <h2>Learned messages</h2>
      <app-paginator :pagination="pagination" />
    </template>
    <template v-if="!isLoadingMessages && !isErrorMessages && messages">
      <v-list density="compact">
        <v-list-item
          v-for="(_, i) of messages?.data"
          :key="i"
          :class="{ 'bg-dark': i % 2 !== 0 }"
        >
          <template #title>
            <p
              class="message-content"
              v-html="renderedMessages[i]"
            />
          </template>
        </v-list-item>
      </v-list>
    </template>
    <v-skeleton-loader
      v-else-if="!isErrorMessages"
      type="list-item-three-line"
    />
    <v-alert
      v-else
      type="error"
      class="text-body-2"
    >
      There was an error loading the messages
    </v-alert>
    <template v-if="pagination.totalPages > 1">
      <app-paginator :pagination="pagination" />
    </template>
  </v-container>
</template>

<script lang="ts">
import { useGetChainAnalytics } from "@/api/analytics";
import { useGetBotGuild } from "@/api/bot";
import { useGetGuildData } from "@/api/data";
import type { PageMeta } from "@/api/common";
import { useAuthStore } from "@/stores/auth";
import { formatBytes, formatNumber, guildIconUrl } from "@/utils/format";
import DOMPurify from "dompurify";
import { marked } from "marked";
import { computed, defineComponent, ref, watch } from "vue";
import { useRoute } from "vue-router";

export default defineComponent({
  setup() {
    const auth = useAuthStore();
    const guildId = (useRoute().params as { guildId: string }).guildId;
    const pagination = ref<PageMeta>({
      page: 1,
      pageSize: 100,
      totalPages: 0,
      totalItems: 0,
    });

    const chainQuery = useGetChainAnalytics(auth.token!, guildId);
    const guildQuery = useGetBotGuild(auth.token!, guildId);
    const messagesQuery = useGetGuildData(auth.token!, guildId, pagination);

    const renderedMessages = computed(() => messagesQuery.data.value?.data?.map((text: string) => {
      const rawHtml = marked(text, { async: false });
      return DOMPurify.sanitize(rawHtml);
    }) ?? []);

    watch(() => pagination.value.page,
      () => {
        messagesQuery.refetch();
      },
      { immediate: true }
    );
    watch(() => pagination.value.pageSize,
      () => {
        messagesQuery.refetch();
      },
      { immediate: true }
    );

    return {
      chain: chainQuery.data,
      isLoadingChain: chainQuery.isLoading,
      isErrorChain: chainQuery.isError,
      guild: guildQuery.data,
      isLoadingGuild: guildQuery.isLoading,
      isErrorGuild: guildQuery.isError,
      messages: messagesQuery.data,
      isLoadingMessages: messagesQuery.isLoading,
      isErrorMessages: messagesQuery.isError,
      pagination,
      renderedMessages,
      discordServerInvite: import.meta.env.VITE_DISCORD_SERVER_INVITE,
    };
  },
  methods: {
    formatBytes,
    formatNumber,
    guildIconUrl,
    getChainAnalytics() {
      const chain = this.chain;
      if (!chain) return null;
      return {
        Gifs: chain.gifs,
        Images: chain.images,
        Videos: chain.videos,
        Messages: chain.messages,
        Words: chain.words,
        Complexity: chain.complexity_score,
      };
    },
  },
});
</script>

<style scoped>
.message-content {
  white-space: normal;
  word-wrap: break-word;
  overflow-wrap: break-word;
}

.bg-dark {
  background-color: #09090989;
}
</style>