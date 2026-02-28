<template>
  <v-container class="pa-4">
    <v-row v-if="!isLoading && !isError" />
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