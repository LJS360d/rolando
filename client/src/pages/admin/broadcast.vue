<template>
  <v-container class="pa-2" min-width="100%">
    <v-form ref="broadcastForm" class="d-flex justify-around" @submit.prevent="onFormSubmit">
      <v-row justify="space-between">
        <v-col cols="6">
          <div class="d-flex flex-wrap ga-3">
            <v-card v-for="guild in guilds" :id="`guild:${guild.id}`" :key="guild.id" :data-name="guild.name"
              max-width="180" width="100%" :prepend-avatar="guildIconUrl(guild.id, guild.icon)">
              <template #title>
                <span class="font-weight-light">{{ guild.name }}</span>
              </template>
              <template #subtitle>
                <span class="text-sm"><b>{{ guild.approximate_member_count }}</b> members</span>
              </template>
              <!-- TODO: Channels -->
              <!--
              <template v-slot:text>

              </template>
              -->
              <template #actions>
                <div class="px-2">
                  <v-switch v-model="selectedGuilds[guild.id]" color="primary" inset />
                </div>
              </template>
            </v-card>
          </div>
        </v-col>
        <v-col id="right-col" cols="5" class="ma-2 h-min sticky top-0">
          <v-row>
            <span>Guilds: <b>{{ selectedGuildsCount }}</b> / <b>{{ guilds?.length }}</b></span>
          </v-row>
          <v-row align="center" class="ga-5">
            <v-btn small outlined color="secondary" @click="toggleAllSelection">
              {{ (selectedGuildsCount === guilds?.length) ? "Deselect" : "Select" }} All
            </v-btn>
            <v-text-field v-model="searchText" label="Search" @input="searchGuild" />
          </v-row>
          <v-row>
            <v-textarea v-model="message" label="Message" rows="6" />
          </v-row>
          <v-row>
            <v-switch v-model="keepAfterSubmit" color="primary" label="Keep after submit" inset />
          </v-row>
          <v-row>
            <v-btn class="w-100" type="submit" color="primary">
              Submit
            </v-btn>
          </v-row>
        </v-col>
      </v-row>
    </v-form>
    <v-snackbar v-model="snackbar.visible" :color="snackbar.color" :timeout="3000" bottom>
      {{ snackbar.message }}
    </v-snackbar>
  </v-container>
</template>

<script lang="ts">
import { broadcastMessage, useGetBotGuildsAll } from '@/api/bot';
import { useAuthStore } from '@/stores/auth';
import { guildIconUrl } from '@/utils/format';
import { ref } from 'vue';

export default {
  setup() {
    const auth = useAuthStore();
    const guildsQuery = useGetBotGuildsAll(auth.token!);
    const snackbar = ref({
      visible: false,
      message: "",
      color: "",
    });
    const selectedGuilds = ref({} as Record<string, string | boolean>);
    const message = ref("");
    const keepAfterSubmit = ref(true);
    const searchText = ref("");
    return {
      guilds: guildsQuery.data,
      selectedGuilds,
      message,
      keepAfterSubmit,
      searchText,
      snackbar,
      token: auth.token!
    };
  },
  computed: {
    selectedGuildsCount() {
      const sel = this.selectedGuilds;
      return Object.values(sel?.value ?? sel ?? {}).filter((v) => v).length;
    },
  },
  methods: {
    guildIconUrl,
    onFormSubmit: async function () {
      if (!(this.message ?? "").trim()) {
        this.snackbar.message = "Message is empty";
        this.snackbar.color = "error";
        this.snackbar.visible = true;
        return;
      }
      try {
        const res = await broadcastMessage(this.token, (this.message ?? ""), this.selectedGuilds);
        if (res.status !== 200) {
          throw new Error("Failed to broadcast message");
        }
        this.snackbar.message = `Message broadcasted to ${this.selectedGuildsCount} guilds`;
        this.snackbar.color = "success";
        this.snackbar.visible = true;
      } catch (error) {
        this.snackbar.message = (error as any).data?.error || "Failed to broadcast message";
        this.snackbar.color = "error";
        this.snackbar.visible = true;
        return;
      }
      if (!this.keepAfterSubmit) {
        this.message = "";
        this.selectedGuilds = {};
      }
    },
    toggleAllSelection() {
      if (Object.keys(this.selectedGuilds).length === this.guilds?.length) {
        this.selectedGuilds = {};
      } else {
        this.selectedGuilds = Object.fromEntries(
          this.guilds?.map((guild) => [guild.id, true]) ?? []
        );
      }
    },
    searchGuild() {
      const searchUpper = this.searchText?.trim().toUpperCase() ?? "";
      this.guilds?.forEach((guild) => {
        const nameUpper = guild.name.toUpperCase();
        const guildElement = document.getElementById(`guild:${guild.id}`)!;
        guildElement.hidden = !nameUpper.includes(searchUpper);
      });
    }
  },
};
</script>

<style scoped>
.text-truncate {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.h-min {
  height: min-content;
}

.sticky {
  position: sticky;
}
</style>
