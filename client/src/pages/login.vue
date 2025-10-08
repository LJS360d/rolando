<template>
  <v-container class="pa-4 h-100">
    <v-container v-if="!error" class="d-flex justify-center align-center h-100">
      <v-progress-circular indeterminate color="primary" size="128" width="12" />
    </v-container>
    <v-container v-else>
      <v-alert type="error" class="text-body-2">
        Login error:
        <div class="my-5 text-h5">
          {{ error }}
        </div>
        <v-btn color="red-500" @click="router.replace('/')">Go back to Home</v-btn>
      </v-alert>
    </v-container>
  </v-container>
</template>

<script setup lang="ts">
import { useAuthStore } from '@/stores/auth';
import { useRouter } from 'vue-router';

const router = useRouter();
const error = ref<string | null>(null);
const fragment = window.location.hash.substring(1);
if (!fragment) {
  error.value = 'Access token not found, make sure your OAUTH2_URL has response_type=token';
}

const params = new URLSearchParams(fragment);
const accessToken = params.get('access_token');
const authStore = useAuthStore();

if (accessToken) {
  fetch('/api/auth/@me', {
    method: 'GET',
    headers: {
      Authorization: accessToken,
    },
  })
    .then(async (res) => {
      const body = await res.json();
      if (res.ok) {
        authStore.setAuth(accessToken, { ...body.user, is_owner: body.is_owner, guilds: body.guilds });
        router.replace('/admin');
      } else {
        error.value = body.error || res.statusText;
      }
    })
    .catch(() => {
      error.value = 'Unexpected error, please report it to the creator on Discord.';
    });
} else if (fragment) {
  const params = new URLSearchParams(fragment);
  const queryObject = Object.fromEntries(params.entries());
  error.value = JSON.stringify(queryObject, null, 2);
}
</script>
