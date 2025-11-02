package events

import (
	"rolando/internal/logger"

	"github.com/bwmarrin/discordgo"
)

// handler for GUILD_UPDATE event
func (h *EventsHandler) onGuildUpdate(s *discordgo.Session, e *discordgo.Event) {
	guildUpdate, ok := e.Struct.(*discordgo.GuildUpdate)
	if !ok {
		return
	}
	oldGuild, err := s.State.Guild(guildUpdate.ID)
	if err != nil {
		logger.Errorf("Failed to fetch guild for guild update event: %v", err)
		return
	}
	h.ChainsService.UpdateChainMeta(oldGuild.ID, map[string]interface{}{"name": guildUpdate.Name})
	logger.Infof("Guild %s updated: %s -> %s", guildUpdate.ID, oldGuild.Name, guildUpdate.Name)
}
