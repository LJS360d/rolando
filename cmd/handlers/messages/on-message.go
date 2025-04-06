package messages

import (
	"rolando/internal/logger"
	"rolando/internal/model"
	"rolando/internal/utils"

	discord "github.com/bwmarrin/discordgo"
)

func (h *MessageHandler) OnMessageCreate(s *discord.Session, m *discord.MessageCreate) {
	if m.Author == nil || m.Author.Bot {
		return // do not process bot messages
	}
	// Access guild and content
	content := m.Content
	guild, err := s.Guild(m.GuildID)

	// Skip processing if no guild (should never happen)
	if err != nil {
		return
	}

	// Fetch/Create chain for the guild
	chain, err := h.ChainsService.GetChain(guild.ID)
	if err != nil {
		logger.Errorf("Failed to fetch chain in '%s': %v", guild.Name, err)
		return
	}

	// Update chain state if message content is valid
	if len(content) > 3 {
		if _, err := h.ChainsService.UpdateChainState(guild.ID, []string{content}); err != nil {
			logger.Errorf("Failed to update chain state in '%s': %v", guild.Name, err)
		}
	}

	// Check if a message can be sent to the channel
	channel, err := s.State.Channel(m.ChannelID)
	if err != nil || !utils.HasGuildTextChannelAccess(s, s.State.User.ID, channel) {
		return
	}

	// Check if the bot is mentioned
	if utils.MentionsUser(m.Message, s.State.User.ID, guild) {
		// Reply when mentioned
		go func() {
			message, err := h.getMessage(chain)
			if err != nil {
				logger.Errorf("Failed to generate text for mention reply in '%s': %v", guild.Name, err)
				return
			}

			if _, err = h.Client.ChannelMessageSendComplex(m.ChannelID, &discord.MessageSend{
				Content: message,
				Reference: &discord.MessageReference{
					MessageID: m.ID,
					ChannelID: m.ChannelID,
					GuildID:   m.GuildID,
				},
			}); err != nil {
				logger.Errorf("Failed to send mention reply in '%s': %v", guild.Name, err)
			}
		}()
		return
	}

	// Randomly decide if bot should reply
	if shouldSendRandomMessage(chain.ReplyRate) {
		go func() {
			message, err := h.getMessage(chain)
			if err != nil {
				logger.Errorf("Failed to generate text for random message in '%s': %v", guild.Name, err)
				return
			}
			if _, err = h.Client.ChannelMessageSend(m.ChannelID, message); err != nil {
				logger.Errorf("Failed to send random message in '%s': %v", guild.Name, err)
			}
		}()
	}
}

// --------------------- Helpers ---------------------------

// Helper method to determine if bot should send a random message
func shouldSendRandomMessage(replyRate int) bool {
	return replyRate == 1 || (replyRate > 1 && utils.GetRandom(1, replyRate) == 1)
}

// Generate a message based on chain probabilities
func (h *MessageHandler) getMessage(chain *model.MarkovChain) (string, error) {
	random := utils.GetRandom(4, 25)
	switch {
	case random <= 21:
		return chain.Talk(random), nil
	case random <= 23:
		return chain.MediaStorage.GetMedia("gif")
	case random <= 24:
		return chain.MediaStorage.GetMedia("image")
	default:
		return chain.MediaStorage.GetMedia("video")
	}
}
