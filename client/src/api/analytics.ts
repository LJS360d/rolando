import { useQuery } from "@tanstack/vue-query";

export interface ChainAnalytics {
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
  trained: boolean;
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
    queryKey: ["/analytics"],
    queryFn: async () => {
      const response = await fetch(`/api/analytics`, {
        headers: {
          Authorization: token
        }
      });
      if (!response.ok) throw new Error(`Failed to fetch chains analytics`);
      return response.json() as Promise<ChainAnalytics[]>;
    },
  });
}