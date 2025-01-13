package services

import (
	"rolando/cmd/log"
	"rolando/cmd/repositories"
	"rolando/cmd/utils"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

type DataFetchService struct {
	Session        *discordgo.Session
	MessageLimit   int
	MaxFetchErrors int
	ChainService   *ChainsService
	messagesRepo   *repositories.MessagesRepository
}

func NewDataFetchService(session *discordgo.Session, chainService *ChainsService, messagesRepo *repositories.MessagesRepository) *DataFetchService {
	return &DataFetchService{
		Session:        session,
		MessageLimit:   750000,
		MaxFetchErrors: 5,
		ChainService:   chainService,
		messagesRepo:   messagesRepo,
	}
}

// FetchAllGuildMessages fetches messages from all accessible channels in the guild.
func (d *DataFetchService) FetchAllGuildMessages(guildID string) ([]string, error) {
	guild, err := d.Session.State.Guild(guildID)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	messageCh := make(chan []string, len(guild.Channels))
	for _, channel := range guild.Channels {
		if !utils.HasGuildTextChannelAccess(d.Session, d.Session.State.User.ID, channel) {
			continue
		}

		wg.Add(1)
		go func(channel *discordgo.Channel) {
			defer wg.Done()
			messages, err := d.fetchChannelMessages(channel)
			if err != nil {
				log.Log.Errorf("Failed to fetch messages for channel #%s: %v", channel.Name, err)
				return
			}
			messageCh <- messages
		}(channel)
	}

	wg.Wait()
	close(messageCh)

	var allMessages []string
	for msgs := range messageCh {
		allMessages = append(allMessages, msgs...)
	}

	log.Log.Infof("Fetched %d messages in guild %s", len(allMessages), guild.Name)
	return allMessages, nil
}

func (d *DataFetchService) fetchChannelMessages(channel *discordgo.Channel) ([]string, error) {
	var messages []string
	var lastMessageID string
	errorCount := 0

	for len(messages) < d.MessageLimit {
		batch, err := d.getMessageBatch(channel.ID, lastMessageID)
		if err != nil {
			errorCount++
			if errorCount > d.MaxFetchErrors {
				log.Log.Warnf("Error limit reached for channel #%s: %v", channel.Name, err)
				break
			}
			continue
		}

		if len(batch) == 0 {
			break
		}
		batchMessages := make([]string, len(batch))
		for i, msg := range batch {
			batchMessages[i] = msg.Content
		}
		go d.ChainService.UpdateChainState(channel.GuildID, batchMessages)
		go d.messagesRepo.AddMessagesToGuild(channel.GuildID, batchMessages)
		messages = append(messages, batchMessages...)
		lastMessageID = batch[len(batch)-1].ID
	}

	log.Log.Infof("Fetched %d messages from channel #%s", len(messages), channel.Name)
	return messages, nil
}

func (d *DataFetchService) getMessageBatch(channelID, lastMessageID string) ([]*discordgo.Message, error) {

	messages, err := d.Session.ChannelMessages(channelID, 100, lastMessageID, "", "")
	if err != nil {
		return nil, err
	}

	var cleanMessages []*discordgo.Message
	for _, msg := range messages {
		if len(strings.Fields(msg.Content)) > 1 || d.containsURL(msg.Content) {
			cleanMessages = append(cleanMessages, msg)
		}
	}
	return cleanMessages, nil
}

func (d *DataFetchService) containsURL(content string) bool {
	return strings.Contains(content, "http://") || strings.Contains(content, "https://")
}
