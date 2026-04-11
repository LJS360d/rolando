package events

import (
	"rolando/internal/logger"

	"github.com/disgoorg/disgo/events"
)

// handler for GUILD_UPDATE event
func (h *EventsHandler) onGuildUpdate(e *events.GuildUpdate) {
	guildID := e.Guild.ID.String()

	// Get the old guild from cache if available
	oldGuild, ok := h.Client.Caches.Guild(e.Guild.ID)
	if !ok {
		logger.Errorf("Failed to fetch old guild for guild update event")
		// Still update the chain meta with new data
		h.ChainsService.UpdateChainMeta(guildID, map[string]interface{}{"name": e.Guild.Name})
		logger.Infof("Guild %s updated to: %s", guildID, e.Guild.Name)
		return
	}

	h.ChainsService.UpdateChainMeta(guildID, map[string]interface{}{"name": e.Guild.Name})
	logger.Infof("Guild %s updated: %s -> %s", guildID, oldGuild.Name, e.Guild.Name)
}
