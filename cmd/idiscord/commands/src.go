package commands

import (
	"rolando/internal/logger"

	"github.com/bwmarrin/discordgo"
)

// implementation of /src command
func (h *SlashCommandsHandler) srcCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	repoURL := "https://github.com/LJS360d/rolando"
	err := h.Client.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: repoURL,
		},
	})
	if err != nil {
		logger.Errorf("Failed to send repo URL response: %v", err)
	}
}
