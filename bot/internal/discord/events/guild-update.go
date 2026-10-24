package events

import (
	"context"
	"rolando/internal/logger"

	"github.com/disgoorg/disgo/events"
)

// handler for GUILD_UPDATE event. Only the name is stored, so other edits
// (icon, boosts, settings) don't open the guild.
func (h *EventsHandler) onGuildUpdate(e *events.GuildUpdate) {
	if e.OldGuild.Name == e.Guild.Name {
		return
	}
	guildID := e.Guild.ID.String()
	if _, err := h.ChainsService.UpdateChainMeta(context.Background(), guildID, map[string]any{"name": e.Guild.Name}); err != nil {
		logger.Errorf("Failed to update name of guild %s: %v", guildID, err)
		return
	}
	logger.Infof("Guild %s updated: %s -> %s", guildID, e.OldGuild.Name, e.Guild.Name)
}
