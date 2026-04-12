package commands

import (
	"context"
	"strconv"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /vc joinrate command
func (h *SlashCommandsHandler) vcJoinRateCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	ctx := context.Background()
	options := i.SlashCommandInteractionData().Options
	var rate *int
	for _, option := range options {
		if option.Name == "rate" && option.Type == discord.ApplicationCommandOptionTypeInt {
			value := int(option.Int())
			rate = &value
			break
		}
	}

	chainDoc, err := h.ChainsService.GetChainConf(ctx, i.GuildID().String())
	if err != nil {
		s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.MessageCreate{
				Content: "Failed to retrieve chain data.",
				Flags:   discord.MessageFlagEphemeral,
			},
		})
		return
	}
	var ratePercent float64
	if rate != nil {
		if !h.checkAdmin(i, "You are not authorized to change the VC join rate.") {
			return
		}
		if _, err := h.ChainsService.UpdateChainMeta(ctx, chainDoc.ID, map[string]interface{}{"vc_join_rate": *rate}); err != nil {
			s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
				Type: discord.InteractionResponseTypeCreateMessage,
				Data: discord.MessageCreate{
					Content: "Failed to update VC join rate.",
					Flags:   discord.MessageFlagEphemeral,
				},
			})
			return
		}
		if *rate == 0 {
			ratePercent = 0
		} else {
			ratePercent = float64(1 / float64(*rate) * 100)
		}
		s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.MessageCreate{
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
	s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeCreateMessage,
		Data: discord.MessageCreate{
			Content: "Current VC join rate is `" + strconv.Itoa(chainDoc.VcJoinRate) + "` (" + strconv.FormatFloat(ratePercent, 'f', 2, 64) + "%)",
		},
	})
}
