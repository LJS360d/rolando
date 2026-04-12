package commands

import (
	"context"
	"fmt"
	"rolando/internal/config"
	"rolando/internal/logger"
	"slices"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /train command
func (h *SlashCommandsHandler) trainCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	ctx := context.Background()
	guildID := i.GuildID().String()

	chainDoc, err := h.ChainsService.GetChainConf(ctx, guildID)
	if err != nil {
		logger.Errorf("Failed to fetch chain document for guild %s: %v", guildID, err)
		return
	}
	const cooldown = 30 * time.Minute

	// Check if the server has been trained before (TrainedAt will be non-zero if set)
	if chainDoc.TrainedAt != nil && !chainDoc.TrainedAt.IsZero() {
		timeSinceTrain := time.Now().Sub(*chainDoc.TrainedAt)

		isOwner := slices.Contains(config.OwnerIDs, i.User().ID.String())
		// bot owners skip the cooldown
		if timeSinceTrain < cooldown && !isOwner {
			// Still in the cooldown period
			remainingTime := cooldown - timeSinceTrain

			// Format the remaining time for the user
			// We only care about hours, minutes, and seconds for a short duration
			remainingStr := fmt.Sprintf("%02d:%02d", int(remainingTime.Minutes()), int(remainingTime.Seconds())%60)

			// Send the cooldown message
			s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
				Type: discord.InteractionResponseTypeCreateMessage,
				Data: discord.MessageCreate{
					Content: "Message fetching was last performed on **" + chainDoc.TrainedAt.Format("02/01/2006 15:04:05") + "**." +
						"\nThe train command has a **30 minutes cooldown** to prevent abuse. Please wait **" + remainingStr + "** before trying again.",
					Flags: discord.MessageFlagEphemeral,
				},
			})
			return
		}
		// Re-train Prompt (Cooldown passed)

		confirmRetrainButton := discord.NewDangerButton("Confirm Re-train", "confirm-train-again")
		trainedAtFormatted := chainDoc.TrainedAt.Format("02/01/2006 15:04:05")

		// re-train confirmation reply
		cnt := `The train command has already been performed at **` + trainedAtFormatted + `**.
By performing it again, you will **delete ALL** the fetched data from this server,
and it will be fetched again in all accessible text channels,
you can use the` + "`/channels`" + ` command to see which are accessible.
If you wish to exclude specific channels, revoke my typing permissions in those channels.

This command can only be performed every **30 minutes**. Are you sure?`
		if err := s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.NewMessageCreate().WithContent(cnt).AddActionRow(confirmRetrainButton).WithEphemeral(true),
		}); err != nil {
			logger.Errorf("Failed to send re-train reply to /train command: %v", err)
		}
		return
	}

	// Create buttons for confirmation and cancellation
	confirmButton := discord.NewPrimaryButton("Confirm", "confirm-train")

	// Create an action row with the buttons
	// Send the reply with buttons
	cnt := `Are you sure you want to use **ALL SERVER MESSAGES** as training data for me?
This will fetch data in all accessible text channels,
you can use the` + "`/channels`" + ` command to see which are accessible.
If you wish to exclude specific channels, revoke my typing permissions in those channels.

This command can only be performed every **30 minutes**. Are you sure?`
	if err := s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeCreateMessage,
		Data: discord.NewMessageCreate().WithContent(cnt).AddActionRow(confirmButton).WithEphemeral(true),
	}); err != nil {
		logger.Errorf("Failed to send reply to /train command: %v", err)
	}
}
