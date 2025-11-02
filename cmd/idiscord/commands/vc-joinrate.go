package commands

import (
	"strconv"

	"github.com/bwmarrin/discordgo"
)

// implementation of /vc joinrate command
func (h *SlashCommandsHandler) vcJoinRateCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	var rate *int
	for _, option := range options {
		if option.Name == "rate" && option.Type == discordgo.ApplicationCommandOptionInteger {
			value := int(option.IntValue())
			rate = &value
			break
		}
	}

	guildID := i.GuildID
	chainDoc, err := h.ChainsService.GetChainDocument(guildID)
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
	var ratePercent float64
	if rate != nil {
		if !h.checkAdmin(i, "You are not authorized to change the VC join rate.") {
			return
		}
		if _, err := h.ChainsService.UpdateChainMeta(chainDoc.ID, map[string]interface{}{"vc_join_rate": *rate}); err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Failed to update VC join rate.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
		if *rate == 0 {
			ratePercent = 0
		} else {
			ratePercent = float64(1 / float64(*rate) * 100)
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Set VC join rate to `" + strconv.Itoa(*rate) + "` (" + strconv.FormatFloat(ratePercent, 'f', 2, 64) + "%)",
			},
		})
		return
	}

	if chainDoc.VcJoinRate == 0 {
		ratePercent = 0
	} else {
		ratePercent = float64(1 / float64(chainDoc.VcJoinRate) * 100)
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Current VC join rate is `" + strconv.Itoa(chainDoc.VcJoinRate) + "` (" + strconv.FormatFloat(ratePercent, 'f', 2, 64) + "%)",
		},
	})
}
