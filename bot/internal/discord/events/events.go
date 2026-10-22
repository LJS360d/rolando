package events

import (
	"context"
	"rolando/internal/discord/services"
	"rolando/internal/logger"
	"slices"
	"strconv"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
)

type EventsHandler struct {
	Client        *bot.Client
	ChainsService *services.ChainsService
}

// Constructor for EventsHandler
func NewEventsHandler(client *bot.Client, chainsService *services.ChainsService) *EventsHandler {
	handler := &EventsHandler{
		Client:        client,
		ChainsService: chainsService,
	}

	return handler
}

// OnEventCreate is now a typed event listener router
func (h *EventsHandler) OnEventCreate(event bot.Event) {
	switch e := event.(type) {
	case *events.GuildJoin:
		h.onGuildCreate(e)
	case *events.GuildLeave:
		h.onGuildDelete(e)
	case *events.GuildUpdate:
		h.onGuildUpdate(e)
	case *events.GuildVoiceStateUpdate:
		h.onVoiceStateUpdate(e)
		// Subscriptions Logs
		// case *events.EntitlementCreate:
		//     h.onEntitlementCreate(e)
		// case *events.SubscriptionCreate:
		//     h.onSubscriptionCreate(e)
		// case *events.SubscriptionUpdate:
		//     h.onSubscriptionUpdate(e)
		// default:
		// 	{
		// 		logger.Debugf("Unhandled event: %T", e)
		// 	}
	}
}

// ---------------------- Helpers ---------------------------

// UpdatePresence shows the cluster-wide guild count on this node's shards.
func UpdatePresence(client *bot.Client, guildCount int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Shards() holds shardsMu; SetPresenceForShard locks it again.
	for _, shard := range slices.Collect(client.ShardManager.Shards()) {
		err := client.SetPresenceForShard(ctx, shard.ShardID(),
			gateway.WithListeningActivity(strconv.Itoa(guildCount)+" servers"),
			gateway.WithOnlineStatus(discord.OnlineStatusOnline),
		)
		if err != nil {
			logger.Errorf("error setting presence on shard %d: %v", shard.ShardID(), err)
		}
	}
}
