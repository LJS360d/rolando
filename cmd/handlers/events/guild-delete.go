package events

import (
	"rolando/internal/logger"

	"github.com/bwmarrin/discordgo"
)

// handler for GUILD_DELETE event
func (h *EventsHandler) onGuildDelete(s *discordgo.Session, e *discordgo.Event) {
	guildDelete, ok := e.Struct.(*discordgo.GuildDelete)
	if !ok {
		return
	}
	logger.Infof("Left guild %s", guildDelete.Name)
	h.ChainsService.DeleteChain(guildDelete.ID)
	UpdatePresence(s)
}
