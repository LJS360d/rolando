package commands

import (
	"rolando/internal/config"
	"rolando/internal/logger"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (h *SlashCommandsHandler) withAdminPermission(cb SlashCommandHandler, msg ...string) SlashCommandHandler {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if !h.checkAdmin(i, msg...) {
			return
		}
		cb(s, i)
	}
}

func (h *SlashCommandsHandler) checkAdmin(i *discordgo.InteractionCreate, msg ...string) bool {
	for _, ownerID := range config.OwnerIDs {
		if i.Member.User.ID == ownerID {
			return true
		}
	}

	perms := i.Member.Permissions
	if perms&discordgo.PermissionAdministrator != 0 {
		return true
	}
	var content string
	if len(msg) > 0 {
		content = strings.Join(msg, "")
	} else {
		content = "You are not authorized to use this command."
	}
	h.Client.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	return false
}

func (h *SlashCommandsHandler) withGuildSubscription(skuId string, cb SlashCommandHandler) SlashCommandHandler {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		canUse := h.guildSubscriptionCheck(s, i, skuId)
		if !canUse {
			premiumButton := &discordgo.Button{
				Style: discordgo.PremiumButton,
				SKUID: skuId,
			}
			actionRow := &discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					premiumButton,
				},
			}
			var content string
			content = "Hey the usage of this command is not free! You can start supporting the project and get premium access by buying a subscription right now"
			if config.PremiumsPageLink != "" {
				content += " or on the website " + config.PremiumsPageLink
			} else {
				content += "."
			}
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    content,
					Components: []discordgo.MessageComponent{*actionRow},
				},
			})
			if err != nil {
				logger.Errorf("Failed to send paywall check premium button response: %v", err)
			}
			return
		}
		cb(s, i)
	}
}

func (h *SlashCommandsHandler) guildSubscriptionCheck(s *discordgo.Session, i *discordgo.InteractionCreate, skuId string) bool {
	if !config.PaywallsEnabled {
		return true
	}
	// pass if the user is a bot owner
	for _, ownerID := range config.OwnerIDs {
		if i.Member.User.ID == ownerID {
			logger.Infof("User %s is an owner, skipping guild subscription check", i.Member.User.GlobalName)
			return true
		}
	}
	logger.Infof("Performing guild subscription check for sku '%s'", skuId)
	guildID := i.GuildID
	chainDoc, err := h.ChainsService.GetChainDocument(guildID)
	if err != nil {
		logger.Errorf("Failed to retrieve chain data for guild %s for subscription check: %v", guildID, err)
		return false
	}
	if chainDoc.Premium {
		return true
	}
	// Check if the guild has the SKU
	for _, ent := range i.Entitlements {
		if ent.SKUID == skuId && ent.EndsAt != nil && ent.EndsAt.After(time.Now()) {
			return true
		}
	}
	return false
}
