package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// implementation of /wipe command
func (h *SlashCommandsHandler) wipeCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	var data string
	for _, option := range options {
		if option.Name == "data" && option.Type == discordgo.ApplicationCommandOptionString {
			data = option.StringValue()
			break
		}
	}

	if data == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You must provide the data to be erased.",
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

	err = h.ChainsService.DeleteTextData(i.GuildID, data)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to delete the specified data.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	chain.Delete(data)
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Deleted `%s`", data),
		},
	})
}
