// src/stores/auth.ts
import { defineStore } from 'pinia';

export interface User {
  id: string;
  username: string;
  avatar: string;
  discriminator: string;
  public_flags: number;
  flags: number;
  banner: string;
  accent_color: number;
  global_name: string;
  avatar_decoration_data: null;
  banner_color: string;
  clan: null;
  primary_guild: null;
  mfa_enabled: boolean;
  locale: string;
  premium_type: number;
  is_owner: boolean;
  guilds: string[];
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: null as string | null,
    user: null as User | null,
  }),
  actions: {
    setAuth(token: string, user: User) {
      this.token = token;
      this.user = user;
    },
    logout() {
      this.token = null;
      this.user = null;
    }
  },
  persist: {
    key: 'auth',
    storage: localStorage,
  },
});
