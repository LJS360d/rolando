import { useQuery } from "@tanstack/vue-query";

export interface BotUser {
  accent_color: number;
  avatar_url: string;
  discriminator: string;
  global_name: string;
  id: string;
  invite_url: string;
  slash_commands: SlashCommand[];
  username: string;
  verified: boolean;
  guilds: number;
}

export interface SlashCommand {
  id: string;
  application_id: string;
  version: string;
  type: number;
  name: string;
  dm_permission: boolean;
  nsfw: boolean;
  description: string;
  options: Option[] | null;
}

export interface Option {
  type: number;
  name: string;
  description: string;
  channel_types: null;
  required: boolean;
  options: null;
  autocomplete: boolean;
  choices: null;
}

export function useGetBotUser() {
  return useQuery({
    queryKey: ["/bot/user"],
    queryFn: async () => {
      const response = await fetch(`/api/bot/user`);
      if (!response.ok) throw new Error("Failed to fetch bot user");
      return response.json() as Promise<BotUser>;
    },
  });
}

export interface BotGuild {
  id: string;
  name: string;
  icon: string;
  owner: boolean;
  permissions: string;
  features: string[];
  approximate_member_count: number;
  approximate_presence_count: number;
}

export function useGetBotGuilds(token: string) {
  return useQuery({
    queryKey: ["/bot/guilds"],
    queryFn: async () => {
      const response = await fetch(`/api/bot/guilds`, {
        headers: {
          Authorization: token
        }
      });
      if (!response.ok) throw new Error("Failed to fetch bot guilds");
      return response.json() as Promise<BotGuild[]>;
    },
  });
}

export function useGetBotGuild(token: string, guildId: string) {
  return useQuery({
    queryKey: [`/bot/guilds/${guildId}`],
    queryFn: async () => {
      const response = await fetch(`/api/bot/guilds/${guildId}`, {
        headers: {
          Authorization: token
        }
      });
      if (!response.ok) throw new Error(`Failed to fetch bot guild ${guildId}`);
      return response.json() as Promise<BotGuild>;
    },
  });
}

export interface BotResources {
  cpu_cores: number;
  memory: BotMemory;
  startup_timestamp_unix: number;
}

export interface BotMemory {
  gc_count: number;
  heap_alloc: number;
  heap_sys: number;
  stack_in_use: number;
  sys: number;
  total_alloc: number;
}

export function useGetBotResources() {
  return useQuery({
    queryKey: ["/bot/resources"],
    queryFn: async () => {
      const response = await fetch(`/api/bot/resources`);
      if (!response.ok) throw new Error("Failed to fetch bot resources");
      return response.json() as Promise<BotResources>;
    },
  });
}

export function leaveGuild(token: string, guildId: string) {
  return fetch(`/api/bot/guilds/${guildId}`, {
    method: "DELETE",
    headers: {
      Authorization: token
    }
  })
}

export function broadcastMessage(token: string, content: string, guilds: Record<string, string | boolean>) {
  const body = {
    content,
    guilds: Object.entries(guilds).map(([id, selected]) => ({
      id,
      channel_id: selected ? "" : undefined
    }))
  }
  return fetch(`/api/bot/broadcast`, {
    method: "POST",
    body: JSON.stringify(body),
    headers: {
      Authorization: token
    }
  })
}