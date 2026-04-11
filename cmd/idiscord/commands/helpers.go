package commands

import (
	"rolando/internal/config"
	"rolando/internal/logger"
	"strings"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func (h *SlashCommandsHandler) withAdminPermission(cb SlashCommandHandler, msg ...string) SlashCommandHandler {
	return func(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
		if !h.checkAdmin(i, msg...) {
			return
		}
		cb(s, i)
	}
}

func (h *SlashCommandsHandler) checkAdmin(i *events.ApplicationCommandInteractionCreate, msg ...string) bool {
	for _, ownerID := range config.OwnerIDs {
		if i.User().ID.String() == ownerID {
			return true
		}
	}

	perms := i.Member().Permissions
	if perms&discord.PermissionAdministrator != 0 {
		return true
	}
	var content string
	if len(msg) > 0 {
		content = strings.Join(msg, "")
	} else {
		content = "You are not authorized to use this command."
	}
	h.Client.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeCreateMessage,
		Data: discord.MessageCreate{
			Content: content,
			Flags:   discord.MessageFlagEphemeral,
		},
	})

	return false
}

func (h *SlashCommandsHandler) withGuildSubscription(skuId snowflake.ID, cb SlashCommandHandler) SlashCommandHandler {
	return func(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
		canUse := h.guildSubscriptionCheck(s, i, skuId)
		if !canUse {
			logger.Warnf("Guild %s is not subscribed to SKU %s", i.GuildID, skuId)
			var content string
			content = "Hey the usage of this command is not free! You can start supporting the project and get premium access by buying a subscription right now"
			if config.PremiumsPageLink != "" {
				content += " or on the website " + config.PremiumsPageLink
			} else {
				content += "."
			}
			err := s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
				Type: discord.InteractionResponseTypeCreateMessage,
				Data: discord.NewMessageCreate().WithContent(content).AddActionRow(discord.ButtonComponent{
					Style: discord.ButtonStylePremium,
					SkuID: skuId,
				}),
			})
			if err != nil {
				logger.Errorf("Failed to send paywall check premium button response: %v", err)
			}
			return
		}
		cb(s, i)
	}
}

func (h *SlashCommandsHandler) guildSubscriptionCheck(s *bot.Client, i *events.ApplicationCommandInteractionCreate, skuId snowflake.ID) bool {
	if !config.PaywallsEnabled {
		return true
	}
	// pass if the user is a bot owner
	for _, ownerID := range config.OwnerIDs {
		if i.User().ID.String() == ownerID {
			logger.Infof("User %s is a bot owner, skipping guild subscription check", *i.User().GlobalName)
			return true
		}
	}
	guildID := i.GuildID().String()
	chainDoc, err := h.ChainsService.GetChainDocument(guildID)
	if err != nil {
		logger.Errorf("Failed to retrieve chain data for guild %s for subscription check: %v", guildID, err)
		return false
	}
	if chainDoc.Premium {
		logger.Infof("Guild '%s' is premium, skipping guild subscription check", chainDoc.Name)
		return true
	}
	logger.Infof("Performing guild subscription check for sku '%s' in guild '%s'", skuId, chainDoc.Name)
	// Check if the guild has the SKU
	for _, ent := range i.Entitlements() {
		if ent.SkuID == skuId && ent.EndsAt != nil && ent.EndsAt.After(time.Now()) {
			return true
		}
	}
	return false
}
