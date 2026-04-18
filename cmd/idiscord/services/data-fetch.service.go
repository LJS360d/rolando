package services

import (
	"context"
	"errors"
	"fmt"
	"rolando/cmd/idiscord/helpers"
	"rolando/internal/logger"
	"rolando/internal/repositories"
	"rolando/internal/utils"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

// concurrent channels being fetched at once
const fetchWorkers = 3

type DataFetchService struct {
	Session        *bot.Client
	MessageLimit   int
	MaxFetchErrors int
	SkipBots       bool // if true, messages from bot accounts are skipped (webhook messages are never skipped)
	ChainService   *ChainsService
	messagesRepo   *repositories.MessagesRepository
}

func NewDataFetchService(session *bot.Client, chainService *ChainsService, messagesRepo *repositories.MessagesRepository) *DataFetchService {
	return &DataFetchService{
		Session:        session,
		MessageLimit:   750000,
		MaxFetchErrors: 5,
		SkipBots:       true,
		ChainService:   chainService,
		messagesRepo:   messagesRepo,
	}
}

// FetchAllGuildMessages fetches messages from all accessible channels in the guild.
func (d *DataFetchService) FetchAllGuildMessages(guildID string) (int, error) {
	gid, err := snowflake.Parse(guildID)
	if err != nil {
		return 0, err
	}

	guild, ok := d.Session.Caches.Guild(gid)
	if !ok {
		return 0, fmt.Errorf("guild with id '%s' not found in cache", guildID)
	}

	var channels []discord.GuildChannel
	d.Session.Caches.ChannelsForGuild(gid)(func(ch discord.GuildChannel) bool {
		if ch.Type() == discord.ChannelTypeGuildText {
			channels = append(channels, ch)
		}
		return true
	})

	// frontload accessible channels
	accessible := channels[:0]
	for _, ch := range channels {
		if helpers.HasGuildTextChannelAccess(d.Session, d.Session.ID(), ch) {
			accessible = append(accessible, ch)
		} else {
			logger.Debugf("channel #%s is not accessible", ch.Name())
		}
	}

	// Worker pool: fetchWorkers goroutines pull from a channel queue.
	// Keeps concurrency bounded so disgo's gateway goroutines aren't starved.
	queue := make(chan discord.GuildChannel, len(accessible))
	for _, ch := range accessible {
		queue <- ch
	}
	close(queue)

	var (
		totalCount atomic.Int64
		wg         sync.WaitGroup
	)

	for range fetchWorkers {
		wg.Go(func() {
			for ch := range queue {
				count, err := d.fetchChannelMessages(ch, guildID)
				if err != nil {
					logger.Errorf("failed to fetch messages for channel #%s: %v", ch.Name(), err)
				}
				totalCount.Add(int64(count))
			}
		})
	}

	wg.Wait()

	total := int(totalCount.Load())
	logger.Infof("fetched %d total messages in guild %s", total, guild.Name)
	return total, nil
}

func (d *DataFetchService) fetchChannelMessages(channel discord.Channel, guildID string) (int, error) {
	var (
		totalFetched int
		lastID       snowflake.ID
		errorCount   int
	)

	for totalFetched < d.MessageLimit {
		raw, cleaned, newLastID, err := d.fetchBatch(channel.ID(), lastID)
		if err != nil {
			if errors.Is(err, rest.ErrNoMorePages) {
				break
			}
			errorCount++
			logger.Warnf("error fetching batch from #%s (attempt %d/%d): %v",
				channel.Name(), errorCount, d.MaxFetchErrors, err)
			if errorCount >= d.MaxFetchErrors {
				break
			}
			time.Sleep(2 * time.Second)
			continue
		}

		// No more pages — API returned nothing new
		if raw == 0 || newLastID == lastID {
			break
		}

		// Only write if the filter produced anything — but always paginate
		if len(cleaned) > 0 {
			d.ChainService.UpdateChainState(context.Background(), guildID, cleaned)
			d.messagesRepo.AddMessagesToGuild(guildID, cleaned)
		}

		totalFetched += raw
		lastID = newLastID
		errorCount = 0

		time.Sleep(300 * time.Millisecond)
	}

	logger.Infof("fetched %d messages from channel #%s", totalFetched, channel.Name())
	return totalFetched, nil
}

// fetchBatch returns: raw message count, cleaned strings, new pagination ID, error.
// Separating raw count from cleaned count is what fixes the false-termination bug.
func (d *DataFetchService) fetchBatch(channelID, lastID snowflake.ID) (int, []string, snowflake.ID, error) {
	messages, err := d.Session.Rest.GetMessages(channelID, 0, lastID, 0, 100)
	if err != nil {
		return 0, nil, 0, err
	}

	if len(messages) == 0 {
		return 0, nil, lastID, nil
	}

	cleaned := d.cleanMessages(messages)
	newLastID := messages[len(messages)-1].ID
	return len(messages), cleaned, newLastID, nil
}

func (d *DataFetchService) cleanMessages(messages []discord.Message) []string {
	var result []string
	for _, msg := range messages {
		isWebhook := msg.WebhookID != nil && *msg.WebhookID != 0
		if d.SkipBots && msg.Author.Bot && !isWebhook {
			continue
		}
		if len(strings.Fields(msg.Content)) > 1 || utils.ReURL.MatchString(msg.Content) {
			result = append(result, msg.Content)
			for _, attachment := range msg.Attachments {
				result = append(result, attachment.URL)
			}
		}
	}
	return result
}
