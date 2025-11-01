import { useQuery } from "@tanstack/vue-query";
import type { Page, PageMeta } from "./common";

export interface ChainAnalytics {
  premium?: boolean;
  bytes: number;
  complexity_score: number;
  gifs: number;
  id: string;
  images: number;
  max_size_mb: number;
  messages: number;
  name: string;
  pings_enabled: boolean;
  reply_rate: number;
  n_gram_size: number;
  vc_join_rate: number;
  reaction_rate: number;
  tts_language: string;
  trained_at: string;
  videos: number;
  words: number;
}

export function useGetChainAnalytics(token: string, chainId: string) {
  return useQuery({
    queryKey: ["/analytics/:chain", chainId],
    queryFn: async () => {
      const response = await fetch(`/api/analytics/${chainId}`, {
        headers: {
          Authorization: token
        }
      });
      if (!response.ok) throw new Error(`Failed to fetch chain ${chainId} analytics`);
      return response.json() as Promise<ChainAnalytics>;
    },
  });
}

export function useGetAllChainsAnalytics(token: string) {
  return useQuery({
    queryKey: ["/analytics/all"],
    queryFn: async () => {
      const response = await fetch(`/api/analytics/all`, {
        headers: {
          Authorization: token
        }
      });
      if (!response.ok) throw new Error(`Failed to fetch chains analytics`);
      return response.json() as Promise<ChainAnalytics[]>;
    },
  });
}

export function useGetChainsAnalyticsPage(token: string, pagination: globalThis.Ref<PageMeta>) {
  return useQuery({
    queryKey: ["/analytics", pagination.value.page, pagination.value.pageSize],
    queryFn: async () => {
      const response = await fetch(`/api/analytics?page=${pagination.value.page}&pageSize=${pagination.value.pageSize}`, {
        headers: {
          Authorization: token
        }
      });
      if (!response.ok) throw new Error("Failed to fetch guild data");
      const res = (await response.json()) as Page<ChainAnalytics[]>;
      pagination.value = res.meta;
      return res;
    },
  });
}
