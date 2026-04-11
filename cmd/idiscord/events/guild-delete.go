package events

import (
	"rolando/internal/logger"

	"github.com/disgoorg/disgo/events"
)

// handler for GUILD_DELETE event (now GuildLeave)
func (h *EventsHandler) onGuildDelete(e *events.GuildLeave) {
	guildID := e.GuildID.String()
	chainDoc, err := h.ChainsService.GetChainDocument(guildID)
	var guildname string
	if err != nil {
		logger.Warnf("Chain document not present for guild %s: %s", guildID, err)
		guildname = guildID
	} else {
		guildname = chainDoc.Name
	}
	logger.Infof("Left guild '%s'", guildname)
	h.ChainsService.DeleteChain(guildID)
	UpdatePresence(h.Client)
}
