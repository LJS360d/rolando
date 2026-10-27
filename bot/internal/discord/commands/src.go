package commands

import (
	"rolando/internal/config"
	"rolando/internal/logger"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /src command
func (h *SlashCommandsHandler) srcCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	err := h.Client.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeCreateMessage,
		Data: discord.MessageCreate{
			Content: config.RepoURL,
		},
	})
	if err != nil {
		logger.Errorf("Failed to send repo URL response: %v", err)
	}
}
