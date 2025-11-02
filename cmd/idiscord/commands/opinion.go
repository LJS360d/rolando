package commands

import (
	"rolando/internal/utils"

	"github.com/bwmarrin/discordgo"
)

// implementation of /opinion command
func (h *SlashCommandsHandler) opinionCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	var about string
	for _, option := range options {
		if option.Name == "about" && option.Type == discordgo.ApplicationCommandOptionString {
			about = option.StringValue()
			break
		}
	}

	if about == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You must provide a word as the seed.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	chain, err := h.ChainsService.GetChain(i.GuildID)
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

	// Generate text with random length between 8 and 40
	msg := chain.GenerateTextFromSeed(about, utils.GetRandom(8, 40))
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
		},
	})
}
