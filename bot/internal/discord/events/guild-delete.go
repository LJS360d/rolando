package events

import (
	"context"
	"rolando/internal/logger"

	"github.com/disgoorg/disgo/events"
)

// handler for GUILD_DELETE event (now GuildLeave)
func (h *EventsHandler) onGuildDelete(e *events.GuildLeave) {
	ctx := context.Background()
	guildID := e.GuildID.String()
	if !h.ChainsService.Guilds.Exists(guildID) {
		logger.Debugf("Left guild '%s' with no local data", guildID)
		return
	}
	guildname := guildID
	if cfg, err := h.ChainsService.Guilds.PeekConfig(guildID); err == nil && cfg.Name != "" {
		guildname = cfg.Name
	}
	logger.Infof("Left guild '%s'", guildname)
	h.ChainsService.DeleteChain(ctx, guildID)
}
