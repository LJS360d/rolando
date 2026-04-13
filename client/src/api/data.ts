import { useQuery } from "@tanstack/vue-query";
import type { Page, PageMeta } from "./common";
import { apiFetch } from "./http";

export function useGetAllGuildData(token: string, guildId: string) {
  return useQuery({
    queryKey: [`/data/${guildId}/all`],
    queryFn: async () => {
      const response = await apiFetch(`/data/${guildId}/all`, { token });
      if (!response.ok) throw new Error("Failed to fetch guild data");
      return response.json() as Promise<string[]>;
    },
  });
}

export function useGetGuildData(token: string, guildId: string, pagination: globalThis.Ref<PageMeta>) {
  return useQuery({
    queryKey: [`/data/${guildId}`, pagination.value.page, pagination.value.pageSize],
    queryFn: async () => {
      const response = await apiFetch(
        `/data/${guildId}?page=${pagination.value.page}&pageSize=${pagination.value.pageSize}`,
        { token },
      );
      if (!response.ok) throw new Error("Failed to fetch guild data");
      const res = (await response.json()) as Page<string[]>;
      pagination.value = res.meta;
      return res;
    },
  });
}
