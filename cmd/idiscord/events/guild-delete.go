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
	chainDoc, err := h.ChainsService.GetChainDocument(guildDelete.ID)
	var guildname string
	if err != nil {
		logger.Warnf("Chain document not present for guild %s: %s", guildDelete.ID, err)
		guildname = guildDelete.ID
	} else {
		guildname = chainDoc.Name
	}
	logger.Infof("Left guild '%s'", guildname)
	h.ChainsService.DeleteChain(guildDelete.ID)
	UpdatePresence(s)
}
