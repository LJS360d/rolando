<template>
  <v-container class="pa-4">
    <v-row v-if="!isLoading && !isError">
      <!-- Hero Section -->
      <v-col
        cols="12"
        lg="6"
      >
        <h1 class="text-h3 font-weight-bold mb-4">
          Meet {{ botUser?.username }}
        </h1>
        <p class="text-body-1 mb-6">
          The learning AI that makes your Discord server dangerously fun.
        </p>
        <v-img
          :src="botUser?.avatar_url"
          alt="AI Discord Bot"
          class="rounded-circle"
          height="200px"
          width="200px"
        />
        <p class="text-body-2 text-gray-500 mt-4">
          Warning: This AI bot learns from messages and can generate NSFW content.
          <br>User discretion is advised.
        </p>
        <h6 class="mt-6">
          Currently part of {{ botUser?.guilds ?? 0 }} servers
        </h6>
        <v-btn
          :href="botUser?.invite_url"
          class="mt-4 bg-discord"
          elevation="2"
        >
          <v-icon
            left
            class="mr-2"
            icon="fa-brands fa-discord"
          />
          Invite to your server
        </v-btn>
      </v-col>

      <!-- Commands Section -->
      <v-col
        cols="12"
        lg="6"
      >
        <v-row class="d-flex align-center">
          <v-col
            cols="12"
            sm="6"
          >
            <h2 class="text-h5 font-weight-semibold">
              Commands
            </h2>
          </v-col>
          <v-col
            cols="12"
            sm="6"
            class="text-right"
          >
            <v-tooltip
              text="Shuffle the commands, why not?"
              location="left"
            >
              <template #activator="{ props }">
                <v-btn
                  v-bind="props"
                  icon
                  @click="shuffleList()"
                >
                  <v-icon icon="fa-solid fa-shuffle" />
                </v-btn>
              </template>
            </v-tooltip>
          </v-col>
        </v-row>

        <v-list
          id="commandsList"
          ref="commandsList"
          v-auto-animate
          class="text-lg"
        >
          <v-list-item
            v-for="(command, index) in botUser?.slash_commands"
            :key="index"
            class="mb-2"
          >
            <v-list-item-title class="font-weight-bold text-h6">
              /{{ command.name }}
            </v-list-item-title>
            <v-list-item-subtitle class="text-body-2">
              {{ command.description }}
            </v-list-item-subtitle>
          </v-list-item>
        </v-list>
      </v-col>
    </v-row>
    <v-progress-circular
      v-else-if="!isError"
      indeterminate
      color="primary"
      size="64"
    />
    <v-alert
      v-else
      type="error"
      class="text-body-2"
    >
      Oops, big error occured, please report it the creator on <a :href="discordServerInvite">the discord</a>
    </v-alert>
  </v-container>
</template>

<script lang="ts">
import { useGetBotUser } from '@/api/bot';
import { defineComponent } from 'vue';

export default defineComponent({
  name: 'IndexPage',
  setup() {
    const botUserQuery = useGetBotUser();

    return {
      discordServerInvite: import.meta.env.VITE_DISCORD_SERVER_INVITE,
      botUser: botUserQuery.data,
      isLoading: botUserQuery.isLoading,
      isError: botUserQuery.isError,
    };
  },
  methods: {
    shuffleList() {
      const list = document.getElementById('commandsList')!;
      for (let i = list.children.length; i >= 0; i--) {
        list?.appendChild(list.children[Math.random() * i | 0]);
      }
    }
  }
});
</script>