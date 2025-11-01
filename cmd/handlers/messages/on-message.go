package messages

import (
	"fmt"
	"math/rand"
	"rolando/cmd/data"
	"rolando/internal/logger"
	"rolando/internal/model"
	"rolando/internal/utils"
	"slices"

	discord "github.com/bwmarrin/discordgo"
)

// OnMessageCreate handles new message events.
func (h *MessageHandler) OnMessageCreate(s *discord.Session, m *discord.MessageCreate) {
	if m.Author == nil || m.Author.Bot {
		return // do not process bot messages
	}
	guild, err := s.Guild(m.GuildID)

	// Skip processing if no guild (should never happen)
	if err != nil {
		return
	}

	channel, err := s.State.Channel(m.ChannelID)
	// Consolidate access check
	if err != nil || !utils.HasGuildTextChannelAccess(s, s.State.User.ID, channel) {
		return
	}

	// 3. Concurrent Chain State Update (Non-Blocking)
	// Update chain state for content and attachments concurrently with other operations.
	go func() {
		// Only fetch/create chainDoc and chain once here, as they are used for all subsequent calls.
		chainDoc, err := h.ChainsService.GetChainDocument(guild.ID)
		if err != nil {
			logger.Errorf("Failed to fetch chain doc in '%s': %v", guild.Name, err)
			return
		}
		chain, err := h.ChainsService.GetChain(guild.ID)
		if err != nil {
			logger.Errorf("Failed to fetch chain in '%s': %v", guild.Name, err)
			return
		}

		messages := make([]string, 0)
		if len(m.Content) > 3 {
			messages = append(messages, m.Content)
		}
		for _, attachment := range m.Attachments {
			if attachment == nil {
				// should never happen
				continue
			}
			messages = append(messages, attachment.URL)
		}

		if len(messages) > 0 {
			if _, err := h.ChainsService.UpdateChainState(guild.ID, messages); err != nil {
				logger.Errorf("Failed to update chain state in '%s': %v", guild.Name, err)
			}
		}

		// Must use the fetched chain/chainDoc from *this* goroutine

		if utils.MentionsUser(m.Message, s.State.User.ID, guild) {
			h.handleReply(s, m.Message, chain)
		}
		if ratedChoice(chain.ReplyRate) {
			h.handleRandomMessage(s, m.ChannelID, guild.Name, chain)
		}
		if ratedChoice(chainDoc.ReactionRate) {
			h.handleReaction(s, m.Message, guild.Name)
		}
	}()
}

// handleReply sends a message in reply to a mention.
func (h *MessageHandler) handleReply(s *discord.Session, m *discord.Message, chain *model.MarkovChain) {
	message, err := h.getMessage(chain)
	if err != nil {
		logger.Errorf("Failed to generate text for mention reply in '%s': %v", m.GuildID, err)
		return
	}

	sendData := &discord.MessageSend{
		Content: message,
		Reference: &discord.MessageReference{
			MessageID: m.ID,
			ChannelID: m.ChannelID,
			GuildID:   m.GuildID,
		},
	}
	if _, err = h.Client.ChannelMessageSendComplex(m.ChannelID, sendData); err != nil {
		logger.Errorf("Failed to send mention reply in '%s': %v", m.GuildID, err)
	}
}

// handleRandomMessage sends a non-reply message.
func (h *MessageHandler) handleRandomMessage(s *discord.Session, channelID, guildName string, chain *model.MarkovChain) {
	message, err := h.getMessage(chain)
	if err != nil {
		logger.Errorf("Failed to generate text for random message in '%s': %v", guildName, err)
		return
	}
	if _, err = h.Client.ChannelMessageSend(channelID, message); err != nil {
		logger.Errorf("Failed to send random message in '%s': %v", guildName, err)
	}
}

// handleReaction adds a random reaction to a message.
func (h *MessageHandler) handleReaction(s *discord.Session, m *discord.Message, guildName string) {
	guildEmojis, err := s.GuildEmojis(m.GuildID)
	if err != nil {
		logger.Errorf("Failed to fetch guild emojis in '%s': %v", guildName, err)
		return
	}

	// base emoji pool
	emojiPool := slices.Clone(data.EmojiUnicodes)
	// add guild custom emojis to the base pool
	for _, emoji := range guildEmojis {
		emojiPool = append(emojiPool, emoji.MessageFormat())
	}

	randEmoji := emojiPool[rand.Intn(len(emojiPool))]
	if err = s.MessageReactionAdd(m.ChannelID, m.ID, randEmoji); err != nil {
		logger.Errorf("Failed to add reaction: %v", err)
	}
}

// --------------------- Helpers ---------------------------

// Helper method to determine if bot should send a commit a rate weighted action
func ratedChoice(rate int) bool {
	return rate == 1 || (rate > 1 && utils.GetRandom(1, rate) == 1)
}

// Generate a message based on chain probabilities
func (h *MessageHandler) getMessage(chain *model.MarkovChain) (string, error) {
	// Generate a random number between 4 and 25 (inclusive).
	random := utils.GetRandom(4, 25)

	switch {
	// (21/22 or approx. 95.5%) to just talk.
	case random <= 21:
		return chain.Talk(random), nil

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
func (h *MessageHandler) tryGetMediaOrTalk(chain *model.MarkovChain, mediaType string, random int) (string, error) {
	var hasMedia bool

	switch mediaType {
	case "gif":
		hasMedia = len(chain.MediaStore.Gifs) > 0
	case "image":
		hasMedia = len(chain.MediaStore.Images) > 0
	case "video":
		hasMedia = len(chain.MediaStore.Videos) > 0
	default:
		return "", fmt.Errorf("unsupported media type: %s", mediaType)
	}

	if hasMedia {
		return chain.MediaStore.GetMedia(mediaType)
	}

	// Fallback to text generation if media is not available.
	return chain.Talk(random), nil
}
