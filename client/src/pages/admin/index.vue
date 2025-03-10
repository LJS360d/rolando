<template>
  <v-container
    class="pa-2"
    min-width="100%"
  >
    <v-card
      flat
      :prepend-avatar="botUser?.avatar_url"
    >
      <template #title>
        <span class="font-weight-light">{{ botUser?.global_name }}</span>
      </template>
      <template #subtitle>
        <span class="text-sm mr-4">Uptime: <b>{{ uptime }}</b></span>
        <span class="text-sm">Currently part of <b>{{ guilds?.length }}</b> guilds</span>
      </template>
      <template #text>
        <memory-usage-bar
          v-if="chains && resources"
          :current="resources?.memory.stack_in_use + resources?.memory.heap_alloc"
          :max="resources?.memory.total_alloc"
          :peak="resources?.memory.sys"
          :blocks="computedBlocks"
        />
      </template>
    </v-card>
    <v-divider class="my-4" />
    <div class="d-flex flex-wrap ga-3">
      <v-card
        v-for="guild in guilds"
        :key="guild.id"
        flat
        class="max-w-card"
        :prepend-avatar="guildIconUrl(guild.id, guild.icon)"
      >
        <v-icon
          v-if="userGuilds.includes(guild.id)"
          class="position-absolute"
          icon="fas fa-star"
          size="12"
          style="top: 4px; left: 4px;"
          title="You are a member of this server"
        />
        <template #title>
          <span
            class="font-weight-light"
            :title="guild.name"
          >{{ guild.name }}</span>
        </template>
        <template #subtitle>
          <span class="text-sm"><b>{{ guild.approximate_member_count }}</b> members</span>
        </template>
        <template #text>
          <div v-if="getChain(guild.id)">
            <v-row
              justify="center"
              class="pa-3 pb-0"
            >
              <span>
                {{ formatBytes(getChain(guild.id)?.bytes ?? 0) }} /
                {{ formatBytes(1024 ** 2 * (getChain(guild.id)?.max_size_mb ?? 0)) }}
              </span>
            </v-row>
            <v-row
              justify="space-between"
              class="pa-3"
            >
              <v-col cols="12">
                <v-row
                  v-for="(field, key) in getAnalyticsForGuild(guild.id)"
                  :key="key"
                  justify="space-between"
                >
                  <span class="text-xs">{{ key }}</span>
                  <span class="text-xs">{{ field }}</span>
                </v-row>
              </v-col>
            </v-row>
          </div>
          <v-row
            v-else
            justify="center"
            class="h-100 align-center"
          >
            No data available
          </v-row>
        </template>
        <template #actions>
          <v-row justify="space-between">
            <v-col cols="8">
              <v-tooltip
                #activator="{ props }"
                text="Invite to server"
                location="bottom"
              >
                <guild-invite-btn
                  :guild-id="guild.id"
                  v-bind="props"
                />
              </v-tooltip>
              <v-tooltip
                #activator="{ props }"
                text="Copy ID"
                location="bottom"
              >
                <v-btn
                  v-bind="props"
                  icon="far fa-copy"
                  size="small"
                  @click="copyToClipboard(guild.id)"
                />
              </v-tooltip>
              <v-tooltip
                #activator="{ props }"
                text="Check data"
                location="bottom"
              >
                <v-btn
                  v-if="!!getChain(guild.id)"
                  v-bind="props"
                  :href="`/data/${guild.id}`"
                  target="_blank"
                  icon="far fa-file-lines"
                  size="small"
                />
              </v-tooltip>
              <template v-if="!!getChain(guild.id)">
                <guild-edit-btn
                  :guild="guild"
                  :chain="getChain(guild.id)!"
                  @confirm="updateChain"
                />
              </template>
            </v-col>
            <v-col
              cols="2"
              class="d-flex justify-end"
            >
              <v-tooltip
                #activator="{ props }"
                text="Leave"
                location="bottom"
              >
                <v-btn
                  v-bind="props"
                  class="justify-self-end"
                  color="red"
                  icon="fas fa-right-from-bracket"
                  size="small"
                  @click="() => openConfirmLeaveGuild(guild.name, guild.id)"
                />
              </v-tooltip>
            </v-col>
          </v-row>
        </template>
      </v-card>
    </div>
    <app-dialog
      :model-value="dialog.visible"
      :message="dialog.text"
      :title="dialog.title"
      @confirm="dialog.confirm"
      @cancel="dialog.cancel"
    />
    <v-snackbar
      v-model="snackbar.visible"
      :color="snackbar.color"
      :timeout="3000"
      bottom
    >
      {{ snackbar.message }}
    </v-snackbar>
  </v-container>
</template>

<script lang="ts">
import { useGetAllChainsAnalytics, type ChainAnalytics } from '@/api/analytics';
import { leaveGuild, updateChainDocument, useGetBotGuilds, useGetBotResources, useGetBotUser } from '@/api/bot';
import { useAuthStore } from '@/stores/auth';
import { formatBytes, formatNumber, formatTime, guildIconUrl } from '@/utils/format';
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';

export default {
  setup() {
    const auth = useAuthStore();
    const botUserQuery = useGetBotUser();
    const botResourcesQuery = useGetBotResources();
    const botGuildsQuery = useGetBotGuilds(auth.token!);
    const chainsQuery = useGetAllChainsAnalytics(auth.token!);
    const snackbar = ref({
      visible: false,
      message: "",
      color: "",
    });
    const dialog = ref({
      visible: false,
      title: "",
      text: "",
      confirm: undefined as (() => Promise<void> | void) | undefined,
      cancel: () => {
        dialog.value.visible = false;
      },
    });
    const elapsedSeconds = ref(0);
    // Watch for changes in the startup time and update elapsedSeconds accordingly
    watch(() => botResourcesQuery.data?.value?.startup_timestamp_unix, (newTime) => {
      if (newTime) {
        elapsedSeconds.value = Math.floor(Date.now() / 1000) - newTime;
      }
    }, { immediate: true });

    onMounted(() => {
      const interval = setInterval(() => {
        elapsedSeconds.value += 1;
      }, 1000);

      onBeforeUnmount(() => clearInterval(interval));
    });

    const uptime = computed(() => formatTime(elapsedSeconds.value));

    return {
      botUser: botUserQuery.data,
      inviteLink: import.meta.env.VITE_DISCORD_SERVER_INVITE,
      guilds: botGuildsQuery.data,
      chains: chainsQuery.data,
      chainsRefetch: chainsQuery.refetch,
      resources: botResourcesQuery.data,
      uptime,
      snackbar,
      dialog,
      token: auth.token!,
      botGuildsQuery,
      windowOpen: window.open,
      userGuilds: auth.user?.guilds ?? [],
    };
  },
  computed: {
    computedBlocks() {
      return this.chains?.map(c => Number(c.bytes)) || [];
    },
  },
  methods: {
    formatBytes,
    guildIconUrl,
    copyToClipboard(text: string) {
      navigator.clipboard.writeText(text)
        .then(() => {
          this.snackbar.visible = true;
          this.snackbar.message = "Copied to clipboard";
          this.snackbar.color = "success";
        })
        .catch(() => {
          this.snackbar.visible = true;
          this.snackbar.message = "Failed to copy to clipboard";
          this.snackbar.color = "error";
        });
    },
    openConfirmLeaveGuild(name: string, id: string) {
      this.dialog.visible = true;
      this.dialog.title = "Leave Guild";
      this.dialog.text = `Are you sure you want to leave '${name}'?`;
      this.dialog.confirm = async () => {
        try {
          const res = await leaveGuild(this.token, id)
          if (res.status !== 204) {
            throw new Error("Failed to leave guild");
          }
          this.snackbar.visible = true;
          this.snackbar.message = `Guild ${id} left successfully`;
          this.snackbar.color = "success";
          this.dialog.visible = false;
          this.botGuildsQuery.refetch();
        } catch {
          this.snackbar.visible = true;
          this.snackbar.message = `Failed to leave guild ${id}`;
          this.snackbar.color = "error";
          this.dialog.visible = false;
        }
      };
    },
    updateChain: async function (chain: globalThis.Ref<ChainAnalytics>) {
      try {
        const res = await updateChainDocument(this.token, chain.value.id, {
          pings: chain.value.pings_enabled,
          trained: chain.value.trained,
          reply_rate: chain.value.reply_rate,
          max_size_mb: chain.value.max_size_mb,
        });
        if (!res.ok) {
          throw new Error("Failed to update chain");
        }
        this.snackbar.visible = true;
        this.snackbar.message = `Successfully updated chain '${chain.value.name}'`;
        this.snackbar.color = "success";
        this.chainsRefetch();
      } catch (error) {
        this.snackbar.visible = true;
        this.snackbar.message = `Failed to update chain '${chain.value.name}': ${error}`;
        this.snackbar.color = "error";
      }

    },
    getChain(guildId: string) {
      return this.chains?.find(c => c.id === guildId);
    },
    getAnalyticsForGuild(guildId: string) {
      const chain = this.chains?.find(c => c.id === guildId);
      if (!chain) return null;
      return {
        Gifs: formatNumber(chain.gifs),
        Images: formatNumber(chain.images),
        Videos: formatNumber(chain.videos),
        Messages: formatNumber(chain.messages),
        Words: formatNumber(chain.words),
        Complexity: formatNumber(chain.complexity_score),
        "Reply Rate": !chain.reply_rate ? "0%" : `${chain.reply_rate} | ${(1 / chain.reply_rate * 100).toPrecision(3)}% `,
        "VC Join Rate": !chain.vc_join_rate ? "0%" : `${chain.vc_join_rate} | ${(1 / chain.vc_join_rate * 100).toPrecision(3)}% `,
      };
    },
  },
};
</script>

<style scoped lang="scss">
.v-card {
  display: flex;
  flex-direction: column;

  .v-card-actions {
    justify-self: end;
  }
}

.max-w-card {
  width: 24%;
}

@media (max-width: 1024px) {
  .max-w-card {
    width: 100%;
  }
}

.w-fit {
  width: fit-content;
}
</style>