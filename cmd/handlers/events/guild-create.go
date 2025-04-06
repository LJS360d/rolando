package events

import (
	"fmt"
	"rolando/internal/logger"

	"github.com/bwmarrin/discordgo"
)

// handler for GUILD_CREATE event
func (h *EventsHandler) onGuildCreate(s *discordgo.Session, e *discordgo.Event) {
	guildCreate, ok := e.Struct.(*discordgo.GuildCreate)
	if !ok {
		return
	}
	logger.Infof("Joined guild %s", guildCreate.Name)
	_, err := h.ChainsService.CreateChain(guildCreate.ID, guildCreate.Name)
	if err != nil {
		logger.Errorf("Error creating chain: %s", err)
		return
	}
	s.ChannelMessage(guildCreate.SystemChannelID, fmt.Sprintf("Hello %s.\nperform the command `/train` to use all the server's messages as training data", guildCreate.Name))
	UpdatePresence(s)
}
