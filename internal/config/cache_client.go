package config

import (
	"time"

	"github.com/valkey-io/valkey-go"
)

// ApplyValkeyClientTuning sets client options for a single-node Valkey used by
// this Discord bot: many guilds in parallel, little overlap on the same guild.
// Call after valkey.ParseURL so URL-derived auth/addresses stay intact.
func ApplyValkeyClientTuning(opt *valkey.ClientOption) {
	if opt == nil {
		return
	}

	opt.PipelineMultiplex = 4

	opt.DisableCache = true

	opt.DisableTCPNoDelay = true

	opt.MaxFlushDelay = 20 * time.Microsecond

	opt.RingScaleEachConn = 11

	opt.ConnWriteTimeout = 30 * time.Second

	opt.BlockingPipeline = 4096

	opt.ClientSetInfo = valkey.DisableClientSetInfo

	if opt.ClientName == "" {
		opt.ClientName = "rolando"
	}

	if opt.Dialer.Timeout == 0 {
		opt.Dialer.Timeout = 5 * time.Second
	}
}
