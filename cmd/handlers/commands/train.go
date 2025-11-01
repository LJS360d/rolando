package commands

import (
	"fmt"
	"rolando/internal/config"
	"rolando/internal/logger"
	"slices"
	"time"

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
	const cooldown = 30 * time.Minute

	isOwner := slices.Contains(config.OwnerIDs, i.Member.User.ID)
	// Check if the server has been trained before (TrainedAt will be non-zero if set) bot owners skip the cooldown
	if chainDoc.TrainedAt != nil && !chainDoc.TrainedAt.IsZero() && !isOwner {
		timeSinceTrain := time.Now().Sub(*chainDoc.TrainedAt)

		if timeSinceTrain < cooldown {
			// Still in the cooldown period
			remainingTime := cooldown - timeSinceTrain

			// Format the remaining time for the user
			// We only care about hours, minutes, and seconds for a short duration
			remainingStr := fmt.Sprintf("%02d:%02d", int(remainingTime.Minutes()), int(remainingTime.Seconds())%60)

			// Send the cooldown message
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Message fetching was last performed on **" + chainDoc.TrainedAt.Format("02/01/2006 15:04:05") + "**." +
						"\nThe train command has a **30 minutes cooldown** to prevent abuse. Please wait **" + remainingStr + "** before trying again.",
					Flags: discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
		// Re-train Prompt (Cooldown passed)

		confirmRetrainButton := &discordgo.Button{
			Label:    "Confirm Re-train",
			Style:    discordgo.DangerButton,
			CustomID: "confirm-retrain",
		}

		retrainActionRow := &discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				confirmRetrainButton,
			},
		}
		trainedAtFormatted := chainDoc.TrainedAt.Format("02/01/2006 15:04:05")

		// re-train confirmation reply
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: `The train command has already been performed at **` + trainedAtFormatted + `**.
By performing it again, you will **delete ALL** the fetched data from this server,
and it will be fetched again in all accessible text channels,
you can use the` + "`/channels`" + ` command to see which are accessible.
If you wish to exclude specific channels, revoke my typing permissions in those channels.

This command can only be performed every **30 minutes**. Are you sure?`,
				Components: []discordgo.MessageComponent{*retrainActionRow},
				Flags:      discordgo.MessageFlagsEphemeral,
			},
		}); err != nil {
			logger.Errorf("Failed to send re-train reply to /train command: %v", err)
		}
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

This command can only be performed every **30 minutes**. Are you sure?`,
			Components: []discordgo.MessageComponent{*actionRow},
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		logger.Errorf("Failed to send reply to /train command: %v", err)
	}
}
