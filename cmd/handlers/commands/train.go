package commands

import (
	"rolando/internal/logger"

	"github.com/bwmarrin/discordgo"
)

// implementation of /train command
func (h *SlashCommandsHandler) trainCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	h.ChainsService.GetChain(i.GuildID)
	chainDoc, err := h.ChainsService.GetChainDocument(i.GuildID)
	if err != nil {
		logger.Errorf("Failed to fetch chain document for guild %s: %v", i.GuildID, err)
		return
	}
	if chainDoc.Trained {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Training already completed for this server.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Create buttons for confirmation and cancellation
	confirmButton := &discordgo.Button{
		Label:    "Confirm",
		Style:    discordgo.PrimaryButton,
		CustomID: "confirm-train",
	}

	// Create an action row with the buttons
	actionRow := &discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			confirmButton,
		},
	}

	// Send the reply with buttons
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: `Are you sure you want to use **ALL SERVER MESSAGES** as training data for me?
This will fetch data in all accessible text channels,
you can use the` + "`/channels`" + ` command to see which are accessible.
If you wish to exclude specific channels, revoke my typing permissions in those channels.
`,
			Components: []discordgo.MessageComponent{*actionRow},
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		logger.Errorf("Failed to send reply to /train command: %v", err)
	}
}
