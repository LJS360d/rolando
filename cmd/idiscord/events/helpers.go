package events

import (
	"rolando/internal/config"
	"rolando/internal/logger"
	"rolando/internal/repositories"
	"slices"
	"time"

	"github.com/bwmarrin/discordgo"
)

func GuildSubscriptionCheck(s *discordgo.Session, member *discordgo.Member, chainDoc *repositories.Chain, skuId string) bool {
	if !config.PaywallsEnabled {
		return true
	}
	// pass if the user is a bot owner
	if slices.Contains(config.OwnerIDs, member.User.ID) {
		logger.Infof("User %s is an owner, skipping guild subscription check (event)", member.User.GlobalName)
		return true
	}

	if chainDoc.Premium {
		return true
	}
	entitlements, err := s.Entitlements(s.State.Application.ID, &discordgo.EntitlementFilterOptions{
		GuildID: member.GuildID,
	})
	if err != nil {
		logger.Errorf("Failed to retrieve entitlements for guild %s for subscription check: %v", member.GuildID, err)
		return false
	}
	// Check if the guild has the SKU
	for _, ent := range entitlements {
		if ent.SKUID == skuId && ent.EndsAt != nil && ent.EndsAt.After(time.Now()) {
			return true
		}
	}
	return false
}
