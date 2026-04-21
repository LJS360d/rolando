package messages

import (
	"context"
	"math/rand"
	"rolando/cmd/idiscord/helpers"
	"rolando/internal/data"
	"rolando/internal/logger"
	"rolando/internal/repositories"
	"rolando/internal/utils"
	"slices"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// OnMessageCreate handles new message events.
func (h *MessageHandler) OnMessageCreate(e *events.MessageCreate) {
	if e.GuildID == nil {
		return // Skip DMs
	}

	m := e.Message
	if m.Author.Bot {
		return // do not process bot messages
	}
	guild, ok := h.Client.Caches.Guild(*m.GuildID)

	// Skip processing if no guild (should never happen)
	if !ok {
		return
	}

	channel, ok := h.Client.Caches.Channel(m.ChannelID)
	// Consolidate access check
	if !ok || !helpers.HasGuildTextChannelAccess(h.Client, h.Client.ID(), channel) {
		return
	}

	// 3. Concurrent Chain State Update (Non-Blocking)
	// Update chain state for content and attachments concurrently with other operations.
	go func() {
		// Only fetch/create chainConf_ and chain once here, as they are used for all subsequent calls.
		chainConf, err := h.ChainsService.GetChainConf(context.Background(), guild.ID.String())
		if err != nil {
			logger.Errorf("Failed to fetch chain in '%s': %v", guild.Name, err)
			return
		}

		messages := make([]string, 0)
		if len(m.Content) > 3 {
			messages = append(messages, m.Content)
		}
		for _, attachment := range m.Attachments {
			if attachment.URL == "" {
				// should never happen
				continue
			}
			messages = append(messages, attachment.URL)
		}

		if len(messages) > 0 {
			if err := h.ChainsService.UpdateChainState(context.Background(), guild.ID.String(), messages); err != nil {
				logger.Errorf("Failed to update chain state in '%s': %v", guild.Name, err)
			}
		}

		// Must use the fetched chain/chainDoc from *this* goroutine
		botMember, _ := h.Client.Caches.Member(guild.ID, h.Client.ID())
		if helpers.MentionsUser(m, botMember) {
			if err := h.Client.Rest.SendTyping(m.ChannelID); err != nil {
				logger.Errorf("Failed to send typing in '%s': %v", guild.Name, err)
			}
			h.handleReply(m, chainConf)
		}
		if ratedChoice(chainConf.ReplyRate) {
			if err := h.Client.Rest.SendTyping(m.ChannelID); err != nil {
				logger.Errorf("Failed to send typing in '%s': %v", guild.Name, err)
			}
			h.handleRandomMessage(m, guild.Name, chainConf)
		}
		if ratedChoice(chainConf.ReactionRate) && helpers.HasGuildAddReactionsPermissions(h.Client, h.Client.ID(), channel) {
			h.handleReaction(m, guild.Name)
		}
	}()
}

// handleReply sends a message in reply to a mention.
func (h *MessageHandler) handleReply(m discord.Message, chain *repositories.ChainConfig) {
	message, err := h.getMessage(chain)
	if err != nil {
		logger.Errorf("Failed to generate text for mention reply in '%s': %v", m.GuildID, err)
		return
	}
	if len(message) == 0 {
		return
	}

	sendData := discord.MessageCreate{
		Content: message,
		MessageReference: &discord.MessageReference{
			MessageID: new(m.ID),
			ChannelID: new(m.ChannelID),
			GuildID:   m.GuildID,
		},
	}
	if _, err = h.Client.Rest.CreateMessage(m.ChannelID, sendData); err != nil {
		logger.Errorf("Failed to send mention reply in '%s': %v", m.GuildID, err)
	}
}

// handleRandomMessage sends a non-reply/quiet-reply message.
func (h *MessageHandler) handleRandomMessage(m discord.Message, guildName string, chain *repositories.ChainConfig) {
	message, err := h.getMessage(chain)
	if err != nil {
		logger.Errorf("Failed to generate text for random message in '%s': %v", guildName, err)
		return
	}
	if message == "" {
		// ignore empty
		return
	}
	if ratedChoice(10) /* 10% */ {
		// the message replies to the original message without pinging the user
		sendData := discord.MessageCreate{
			Content: message,
			MessageReference: &discord.MessageReference{
				MessageID: new(m.ID),
				ChannelID: new(m.ChannelID),
				GuildID:   m.GuildID,
			},
			AllowedMentions: &discord.AllowedMentions{
				Parse: []discord.AllowedMentionType{
					discord.AllowedMentionTypeUsers,
					discord.AllowedMentionTypeRoles,
					discord.AllowedMentionTypeEveryone,
				},
				RepliedUser: false,
			},
		}
		if _, err = h.Client.Rest.CreateMessage(m.ChannelID, sendData); err != nil {
			logger.Errorf("Failed to send mention reply in '%s': %v", m.GuildID, err)
		}
		return
	}
	if _, err = h.Client.Rest.CreateMessage(m.ChannelID, discord.MessageCreate{
		Content: message,
	}); err != nil {
		logger.Errorf("Failed to send random message in '%s': %v", guildName, err)
	}
}

// handleReaction adds a random reaction to a message.
func (h *MessageHandler) handleReaction(m discord.Message, guildName string) {
	guildEmojis := h.Client.Caches.Emojis((*m.GuildID))

	// base emoji pool
	emojiPool := slices.Clone(data.EmojiUnicodes)
	// add guild custom emojis to the base pool
	for emoji := range guildEmojis {
		emojiPool = append(emojiPool, emoji.Name)
	}

	randEmoji := emojiPool[rand.Intn(len(emojiPool))]
	if err := h.Client.Rest.AddReaction(m.ChannelID, m.ID, randEmoji); err != nil {
		logger.Errorf("Failed to add reaction: %v", err)
	}
}

// --------------------- Helpers ---------------------------

// Helper method to determine if bot should send a commit a rate weighted action
func ratedChoice(rate int) bool {
	return rate == 1 || (rate > 1 && utils.GetRandom(1, rate) == 1)
}

// Generate a message based on chain probabilities
func (h *MessageHandler) getMessage(chain *repositories.ChainConfig) (string, error) {
	// Generate a random number between 4 and 25 (inclusive).
	random := utils.GetRandom(4, 25)

	switch {
	// (21/22 or approx. 95.5%) to just talk.
	case random <= 21:
		{
			msg, err := h.ChainsService.Generate(context.Background(), chain.ID, random, chain.NGramSize)
			if err != nil {
				return "", err
			}
			return msg, nil
		}

	// (2/22 or approx. 9.1%) for a GIF
	case random <= 23:
		return h.tryGetMediaOrTalk(chain, "gif", random)

	// (1/22 or approx. 4.5%) for an Image
	case random <= 24:
		return h.tryGetMediaOrTalk(chain, "image", random)

	// (1/22 or approx. 4.5%) for a Video
	default:
		return h.tryGetMediaOrTalk(chain, "video", random)
	}
}

// tryGetMediaOrTalk attempts to retrieve a specific type of media;
// if unavailable, it falls back to generating a text message.
func (h *MessageHandler) tryGetMediaOrTalk(chain *repositories.ChainConfig, mediaType string, random int) (string, error) {
	ctx := context.Background()
	media, err := h.ChainsService.GetRandomMedia(ctx, chain.ID, mediaType)
	if err != nil {
		return "", err
	}
	if media != "" {
		return media, nil
	}

	// Fallback to text generation if media is not available.
	msg, err := h.ChainsService.Generate(ctx, chain.ID, random, chain.NGramSize)
	if err != nil {
		return "", err
	}
	return msg, nil
}
