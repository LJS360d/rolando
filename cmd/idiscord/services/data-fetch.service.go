package services

import (
	"context"
	"errors"
	"fmt"
	"rolando/cmd/idiscord/helpers"
	"rolando/internal/logger"
	"rolando/internal/repositories"
	"strings"
	"sync"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

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
func (d *DataFetchService) FetchAllGuildMessages(guildID string) ([]string, error) {
	gid, err := snowflake.Parse(guildID)
	if err != nil {
		return nil, err
	}

	guild, ok := d.Session.Caches.Guild(gid)
	if !ok {
		return nil, fmt.Errorf("guild with id '%s' not found in cache", guildID)
	}

	var channels []discord.GuildChannel
	d.Session.Caches.ChannelsForGuild(gid)(func(ch discord.GuildChannel) bool {
		if ch.Type() == discord.ChannelTypeGuildText {
			channels = append(channels, ch)
		}
		return true // continue iterating
	})

	var (
		wg        sync.WaitGroup
		messageCh = make(chan []string, len(channels))
	)

	for _, channel := range channels {
		if !helpers.HasGuildTextChannelAccess(d.Session, d.Session.ID(), channel) {
			logger.Debugf("channel #%s is not accessible", channel.Name())
			continue
		}
		wg.Add(1)
		go func(ch discord.Channel) {
			defer wg.Done()
			messages, err := d.fetchChannelMessages(ch, guildID)
			if err != nil {
				logger.Errorf("failed to fetch messages for channel #%s: %v", ch.Name(), err)
				return
			}
			if len(messages) > 0 {
				messageCh <- messages
			}
		}(channel)
	}

	wg.Wait()
	close(messageCh)

	var allMessages []string
	for msgs := range messageCh {
		allMessages = append(allMessages, msgs...)
	}

	logger.Infof("fetched %d total messages in guild %s", len(allMessages), guild.Name)
	return allMessages, nil
}

func (d *DataFetchService) fetchChannelMessages(channel discord.Channel, guildID string) ([]string, error) {
	var (
		allMessages []string
		lastID      snowflake.ID // zero value = start from latest
		errorCount  int
	)

	for len(allMessages) < d.MessageLimit {
		batch, newLastID, err := d.fetchBatch(channel.ID(), lastID)
		if err != nil {
			if errors.Is(err, rest.ErrNoMorePages) {
				break // clean end of channel history, not an error
			}
			errorCount++
			logger.Warnf("error fetching batch from #%s (attempt %d/%d): %v", channel.Name(), errorCount, d.MaxFetchErrors, err)
			if errorCount >= d.MaxFetchErrors {
				logger.Warnf("error limit reached for channel #%s, stopping", channel.Name())
				break
			}
			continue
		}

		if newLastID == lastID {
			break
		}

		go d.ChainService.UpdateChainState(context.Background(), guildID, batch)
		go d.messagesRepo.AddMessagesToGuild(guildID, batch)

		allMessages = append(allMessages, batch...)
		lastID = newLastID
		errorCount = 0 // reset on success
	}

	logger.Infof("fetched %d messages from channel #%s", len(allMessages), channel.Name())
	return allMessages, nil
}

// fetchBatch fetches one page of up to 100 messages before lastID (or from the latest if lastID is zero).
// Returns the cleaned message strings, the ID of the last (oldest) message fetched for pagination, and any error.
func (d *DataFetchService) fetchBatch(channelID, lastID snowflake.ID) ([]string, snowflake.ID, error) {
	messages, err := d.Session.Rest.GetMessages(channelID, 0, lastID, 0, 100)
	if err != nil {
		return nil, 0, err
	}

	if len(messages) == 0 {
		return nil, lastID, nil
	}

	cleaned := d.cleanMessages(messages)
	newLastID := messages[len(messages)-1].ID
	return cleaned, newLastID, nil
}

func (d *DataFetchService) cleanMessages(messages []discord.Message) []string {
	var result []string
	for _, msg := range messages {
		isWebhook := msg.WebhookID != nil && *msg.WebhookID != 0
		if d.SkipBots && msg.Author.Bot && !isWebhook {
			continue
		}
		if len(strings.Fields(msg.Content)) > 1 || d.containsURL(msg.Content) {
			result = append(result, msg.Content)
			for _, attachment := range msg.Attachments {
				result = append(result, attachment.URL)
			}
		}
	}
	return result
}

func (d *DataFetchService) containsURL(content string) bool {
	return strings.Contains(content, "http://") || strings.Contains(content, "https://")
}
