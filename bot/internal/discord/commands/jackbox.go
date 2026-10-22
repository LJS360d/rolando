package commands

import (
	"context"
	"strings"

	"rolando/internal/logger"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func (h *SlashCommandsHandler) jackboxCommand(client *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	if h.Jackbox == nil {
		_ = client.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.MessageCreate{
				Content: "Jackbox integration is not configured.",
				Flags:   discord.MessageFlagEphemeral,
			},
		})
		return
	}
	if i.GuildID() == nil {
		_ = client.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.MessageCreate{
				Content: "This command is only available in a server.",
				Flags:   discord.MessageFlagEphemeral,
			},
		})
		return
	}

	err := i.DeferCreateMessage(true)
	if err != nil {
		logger.Errorf("jackbox defer: %v", err)
		return
	}

	data := i.SlashCommandInteractionData()
	code := strings.TrimSpace(data.String("code"))

	guildID := i.GuildID().String()

	if code == "" {
		h.Jackbox.Stop(guildID)
		_, err = client.Rest.UpdateInteractionResponse(client.ApplicationID, i.Token(), discord.NewMessageUpdate().WithContent("Jackbox session cleared for this server."))
		if err != nil {
			logger.Errorf("jackbox update response: %v", err)
		}
		return
	}

	ctx := context.Background()
	appTag, err := h.Jackbox.Start(ctx, guildID, code)
	if err != nil {
		_, err = client.Rest.UpdateInteractionResponse(client.ApplicationID, i.Token(), discord.NewMessageUpdate().WithContent("Could not join: "+err.Error()))
		if err != nil {
			logger.Errorf("jackbox update response: %v", err)
		}
		return
	}

	room := strings.ToUpper(strings.TrimSpace(code))
	msg := "Joined Jackbox room **" + room + "** (`" + appTag + "`)."
	_, err = client.Rest.UpdateInteractionResponse(client.ApplicationID, i.Token(), discord.NewMessageUpdate().WithContent(msg))
	if err != nil {
		logger.Errorf("jackbox update response: %v", err)
	}
}
