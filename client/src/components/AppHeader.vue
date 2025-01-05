<template>
  <v-app-bar app>
    <v-app-bar-nav-icon
      icon="fas fa-bars"
      @click="drawer = !drawer"
    />

    <v-tooltip
      text="Join the discord"
      location="bottom"
    >
      <template #activator="{ props }">
        <a
          v-bind="props"
          :href="DISCORD_SERVER_INVITE"
        >
          <v-app-bar-nav-icon icon="fa-brands fa-discord" />
        </a>
      </template>
    </v-tooltip>

    <v-spacer />

    <v-tooltip
      v-if="isLoggedIn"
      text="Click to Logout"
      location="bottom"
    >
      <template #activator="{ props }">
        <div
          v-bind="props"
          class="cursor-pointer pa-1 mr-2 rounded"
          @click="logout"
        >
          <v-avatar
            class="mr-3"
            :size="40"
          >
            <img
              :src="userAvatarUrl"
              alt="User Avatar"
            >
          </v-avatar>
          <span>{{ userDisplayName }}</span>
        </div>
      </template>
    </v-tooltip>
    <v-btn
      v-else
      class="ml-3 bg-discord"
      :href="OAUTH2_URL"
      target="_self"
    >
      <v-icon
        class="mr-2"
        icon="fa-brands fa-discord"
      />
      Login with Discord
    </v-btn>
  </v-app-bar>

  <v-navigation-drawer
    v-model="drawer"
    app
    width="250"
  >
    <v-list>
      <v-list-item
        v-for="(item, index) in links"
        :key="index"
        @click="$router.push(item.href)"
      >
        <span>
          <v-icon
            :v-if="item.icon"
            class="mx-2"
            size="16"
            :icon="item.icon"
          />
          <span class="text-body-2">{{ item.name }}</span>
        </span>
      </v-list-item>
      <template v-if="isOwner">
        <v-divider />
        <v-list-item
          v-for="(item, index) in adminLinks"
          :key="index"
          @click="$router.push(item.href)"
        >
          <span>
            <v-icon
              :v-if="item.icon"
              class="mx-2"
              size="16"
              :icon="item.icon"
            />
            <span class="text-body-2">{{ item.name }}</span>
          </span>
        </v-list-item>
      </template>
    </v-list>
  </v-navigation-drawer>
</template>

<script lang="ts">
import { defineComponent, ref, computed } from "vue";
import { useAuthStore } from "@/stores/auth";

interface MenuItem {
  name: string;
  href: string;
  icon?: string;
}

const OAUTH2_URL: string = import.meta.env.VITE_OAUTH2_URL;
const DISCORD_SERVER_INVITE: string = import.meta.env.VITE_DISCORD_SERVER_INVITE;

export default defineComponent({
  name: "AppHeader",
  setup() {
    const authStore = useAuthStore();
    const drawer = ref(false);
    const links = ref<MenuItem[]>([
      { name: "Home", icon: "fas fa-home", href: "/" },
      { name: "Privacy Policy", icon: "fas fa-user-shield", href: "/privacy-policy" },
      { name: "Terms of Service", icon: "fas fa-file-signature", href: "/terms-of-service" },
    ]);
    const adminLinks = ref<MenuItem[]>([
      { name: "Admin Panel", icon: "fas fa-cogs", href: "/admin" },
      { name: "Message Broadcast", icon: "fas fa-bullhorn", href: "/admin/broadcast" },
    ]);

    const isLoggedIn = computed(() => !!authStore.token);
    const userAvatarUrl = computed(() =>
      authStore.user?.avatar
        ? `https://cdn.discordapp.com/avatars/${authStore.user.id}/${authStore.user.avatar}.png?size=40`
        : "https://cdn.discordapp.com/embed/avatars/0.png"
    );

    const userDisplayName = computed(() =>
      authStore.user?.username || authStore.user?.global_name || ""
    );

    return {
      drawer,
      links,
      adminLinks,
      logout: authStore.logout,
      isLoggedIn,
      isOwner: computed(() => authStore.user?.is_owner || false),
      userAvatarUrl,
      userDisplayName,
      OAUTH2_URL,
      DISCORD_SERVER_INVITE,
    };
  },
});
</script>
