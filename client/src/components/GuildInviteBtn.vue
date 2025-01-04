<template>
  <v-btn :icon="buttonIcon" :color="buttonColor" :href="inviteLink" target="_blank" size="small"
    @click="getInvite"></v-btn>
</template>

<script lang="ts">
import { useAuthStore } from '@/stores/auth';
import { ref } from 'vue';

const buttonIcon = ref('fas fa-door-closed');
const buttonColor = ref('');
const inviteLink = ref('');

export default defineComponent({
  name: 'GuildInviteBtn',
  props: {
    guildId: {
      type: String,
      required: true,
    },
  },
  data() {
    const auth = useAuthStore();
    return {
      token: auth.token!,
      inviteLink,
      buttonIcon,
      buttonColor
    }
  },
  methods: {
    getInvite: async function () {
      try {
        const response = await fetch(`/api/bot/guilds/${this.guildId}/invite`,
          {
            headers: {
              Authorization: this.token
            }
          }
        );

        if (!response.ok) {
          throw new Error('Failed to fetch invite');
        }

        const data = await response.json();

        if (data && data.invite) {
          buttonIcon.value = 'fas fa-door-open';
          buttonColor.value = 'green';
          inviteLink.value = data.invite;
        }
      } catch (error) {
        console.error('Error fetching invite:', error);
        buttonColor.value = 'red';
      }
    },
  }
})

</script>
