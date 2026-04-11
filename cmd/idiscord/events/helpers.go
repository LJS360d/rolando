package events

import (
	"rolando/internal/config"
	"rolando/internal/logger"
	"rolando/internal/repositories"
	"slices"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

func GuildSubscriptionCheck(client *bot.Client, member discord.Member, chainDoc *repositories.Chain, skuId snowflake.ID) bool {
	if !config.PaywallsEnabled {
		return true
	}
	// pass if the user is a bot owner
	if slices.Contains(config.OwnerIDs, member.User.ID.String()) {
		logger.Infof("User %s is an owner, skipping guild subscription check (event)", member.User.EffectiveName())
		return true
	}

	if chainDoc.Premium {
		return true
	}

	// Get entitlements for the guild
	entitlements, err := client.Rest.GetEntitlements(client.ApplicationID, rest.GetEntitlementsParams{
		GuildID: member.GuildID,
	})
	if err != nil {
		logger.Errorf("Failed to retrieve entitlements for guild %s for subscription check: %v", member.GuildID.String(), err)
		return false
	}

	// Check if the guild has the SKU
	for _, ent := range entitlements {
		if ent.SkuID == skuId && ent.EndsAt != nil && ent.EndsAt.After(time.Now()) {
			return true
		}
	}
	return false
}
