<template>
  <div v-if="ready" class="memory-usage-stack">
    <div class="d-flex align-center justify-space-between flex-wrap ga-2 mb-2">
      <span class="text-body-2 text-medium-emphasis">Resources</span>
      <v-chip size="small" variant="tonal" color="secondary">
        {{ resources.cpu_cores }} CPU
      </v-chip>
    </div>

    <div
      v-if="layout.mode === 'fallback'"
      class="text-caption text-medium-emphasis mb-1"
    >
      Host RAM unavailable — bar normalized to chain index + RSS only
    </div>

    <div class="d-flex align-center ga-2 mb-1">
      <div
        class="unified-track mem-row--interactive rounded overflow-hidden flex-grow-1"
        style="height: 28px; min-width: 0"
      >
        <template v-if="layout.segments.length === 0">
          <div class="unified-track__empty w-100 h-100 d-flex align-center justify-center text-caption">
            No chain or RSS data
          </div>
        </template>
        <template v-else>
          <div class="unified-track__inner d-flex h-100 w-100">
            <v-tooltip
              v-for="(seg, index) in layout.segments"
              :key="`${seg.kind}-${index}`"
              :text="`${seg.tooltipTitle} | ${seg.mbLabel}`"
              location="top"
            >
              <template #activator="{ props: tip }">
                <div
                  v-bind="tip"
                  class="unified-track__seg h-100"
                  :class="seg.kind === 'rss' ? 'unified-track__seg--rss' : ''"
                  :style="{
                    flex: `0 0 ${seg.displayPct}%`,
                    minWidth: seg.displayPct > 0 ? '2px' : '0',
                    backgroundColor:
                      seg.kind === 'rss' ? undefined : chainColor(seg.chainColorIndex),
                  }"
                />
              </template>

            </v-tooltip>
            <div
              v-if="layout.freePct > 0.01"
              class="unified-track__free h-100"
              :style="{ flex: `0 0 ${layout.freePct}%`, minWidth: 0 }"
            />
          </div>
        </template>
      </div>
      <span
        v-if="layout.mode === 'host' && layout.hostTotal > 0"
        class="text-caption text-medium-emphasis unified-track__cap flex-shrink-0"
      >
        {{ formatBytes(layout.hostTotal) }}
      </span>
    </div>
  </div>
  <v-progress-linear
    v-else
    indeterminate
    height="8"
    color="grey-lighten-2"
    rounded
    class="rounded-lg"
  />
</template>

<script lang="ts">
import type { BotResources } from "@/api/bot";
import { formatBytes } from "@/utils/format";
import { defineComponent, type PropType } from "vue";

export interface ChainSegment {
  name: string;
  bytes: number;
}

type BarSeg = {
  kind: "chain" | "rss";
  name: string;
  bytes: number;
  displayPct: number;
  tooltipTitle: string;
  mbLabel: string;
  chainColorIndex: number;
};

type Layout =
  | {
      mode: "host";
      hostTotal: number;
      segments: BarSeg[];
      freePct: number;
    }
  | {
      mode: "fallback";
      hostTotal: 0;
      segments: BarSeg[];
      freePct: number;
    };

function formatMb(bytes: number): string {
  const mb = bytes / (1024 * 1024);
  return `${mb.toFixed(2)} MB`;
}

export default defineComponent({
  name: "MemoryUsageBar",
  props: {
    resources: {
      type: Object as PropType<BotResources>,
      required: true,
    },
    chainSegments: {
      type: Array as PropType<ChainSegment[]>,
      required: false,
      default: () => [],
    },
  },
  computed: {
    ready(): boolean {
      return !!this.resources?.process;
    },
    hostTotal(): number {
      const h = this.resources.host as { total_bytes?: number };
      return typeof h.total_bytes === "number" ? h.total_bytes : 0;
    },
    processRss(): number {
      return Math.max(0, this.resources.process.rss_bytes || 0);
    },
    chainSum(): number {
      return this.chainSegments.reduce((a, s) => a + Number(s.bytes || 0), 0);
    },
    layout(): Layout {
      const rss = this.processRss;
      const chains = this.chainSegments;
      const h = this.hostTotal;

      const buildTooltips = (
        kind: "chain" | "rss",
        name: string,
        bytes: number,
      ) => {
        if (kind === "chain") {
          return {
            tooltipTitle: name,
            mbLabel: formatMb(bytes),
          };
        }
        return {
          tooltipTitle: "Backend process (RSS)",
          mbLabel: `${formatMb(bytes)} resident`,
        };
      };

      if (h > 0) {
        const rawItems: {
          kind: "chain" | "rss";
          name: string;
          bytes: number;
          rawPct: number;
          chainIdx: number;
        }[] = [];

        chains.forEach((c, chainIdx) => {
          const b = Number(c.bytes) || 0;
          if (b <= 0) return;
          rawItems.push({
            kind: "chain",
            name: c.name || "Guild",
            bytes: b,
            rawPct: (b / h) * 100,
            chainIdx,
          });
        });

        if (rss > 0) {
          rawItems.push({
            kind: "rss",
            name: "RSS",
            bytes: rss,
            rawPct: (rss / h) * 100,
            chainIdx: -1,
          });
        }

        if (rawItems.length === 0) {
          return { mode: "host", hostTotal: h, segments: [], freePct: 100 };
        }

        const sumRaw = rawItems.reduce((a, x) => a + x.rawPct, 0);
        const scale = sumRaw > 100 ? 100 / sumRaw : 1;

        const segments: BarSeg[] = rawItems.map((x) => {
          const t = buildTooltips(x.kind, x.name, x.bytes);
          return {
            kind: x.kind,
            name: x.name,
            bytes: x.bytes,
            displayPct: x.rawPct * scale,
            tooltipTitle: t.tooltipTitle,
            mbLabel: t.mbLabel,
            chainColorIndex: x.chainIdx,
          };
        });

        const used = segments.reduce((a, s) => a + s.displayPct, 0);
        const freePct = Math.max(0, 100 - used);

        return {
          mode: "host",
          hostTotal: h,
          segments,
          freePct,
        };
      }

      const sumBytes = this.chainSum + rss;
      const denom = Math.max(sumBytes, 1);
      const rawItems: {
        kind: "chain" | "rss";
        name: string;
        bytes: number;
        rawPct: number;
        chainIdx: number;
      }[] = [];

      chains.forEach((c, chainIdx) => {
        const b = Number(c.bytes) || 0;
        if (b <= 0) return;
        rawItems.push({
          kind: "chain",
          name: c.name || "Guild",
          bytes: b,
          rawPct: (b / denom) * 100,
          chainIdx,
        });
      });
      if (rss > 0) {
        rawItems.push({
          kind: "rss",
          name: "RSS",
          bytes: rss,
          rawPct: (rss / denom) * 100,
          chainIdx: -1,
        });
      }

      if (rawItems.length === 0) {
        return { mode: "fallback", hostTotal: 0, segments: [], freePct: 100 };
      }

      const segments: BarSeg[] = rawItems.map((x) => {
        const t = buildTooltips(x.kind, x.name, x.bytes);
        return {
          kind: x.kind,
          name: x.name,
          bytes: x.bytes,
          displayPct: x.rawPct,
          tooltipTitle: t.tooltipTitle,
          mbLabel: t.mbLabel,
          chainColorIndex: x.chainIdx,
        };
      });

      const used = segments.reduce((a, s) => a + s.displayPct, 0);
      const freePct = Math.max(0, 100 - used);

      return {
        mode: "fallback",
        hostTotal: 0,
        segments,
        freePct,
      };
    },
  },
  methods: {
    formatBytes,
    chainColor(index: number): string {
      if (index < 0) return "rgba(0, 150, 136, 0.9)";
      const colors = [
        "rgba(76, 175, 80, 0.85)",
        "rgba(255, 193, 7, 0.85)",
        "rgba(244, 67, 54, 0.85)",
        "rgba(33, 150, 243, 0.85)",
        "rgba(156, 39, 176, 0.85)",
      ];
      return colors[index % colors.length];
    },
  },
});
</script>

<style scoped>
.memory-usage-stack {
  max-width: 100%;
}

.mem-row--interactive {
  transition: transform 0.15s ease, filter 0.15s ease;
  padding: 2px 0;
}

.mem-row--interactive:hover {
  transform: translateY(-1px);
  filter: brightness(1.04);
}

.unified-track {
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.35);
  background: rgba(0, 0, 0, 0.14);
}

.unified-track__inner {
  min-height: 100%;
}

.unified-track__seg {
  transition: filter 0.15s ease;
}

.unified-track__seg:hover {
  filter: brightness(1.12) saturate(1.1);
}

.unified-track__seg--rss {
  background: linear-gradient(
    180deg,
    rgb(0, 131, 143) 0%,
    rgb(0, 105, 117) 100%
  ) !important;
}

.unified-track__free {
  background: rgba(0, 0, 0, 0.1);
}

.unified-track__empty {
  background: rgba(0, 0, 0, 0.12);
}
</style>
