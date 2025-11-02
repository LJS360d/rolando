package commands

import "github.com/bwmarrin/discordgo"

// implementation of /togglepings command
func (h *SlashCommandsHandler) togglePingsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	guildID := i.GuildID
	chain, err := h.ChainsService.GetChain(guildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to retrieve chain data.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if _, err := h.ChainsService.UpdateChainMeta(guildID, map[string]interface{}{"pings": !chain.Pings}); err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to toggle pings state.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	state := "disabled"
	if chain.Pings {
		state = "enabled"
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Pings are now `" + state + "`",
		},
	})
}
