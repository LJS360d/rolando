package data

import (
	"rolando/cmd/repositories"
	"rolando/server/auth"
	"strconv"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
)

type DataController struct {
	messagesRepo *repositories.MessagesRepository
	ds           *discordgo.Session
}

func NewController(ds *discordgo.Session, messagesRepo *repositories.MessagesRepository) *DataController {
	return &DataController{
		messagesRepo: messagesRepo,
		ds:           ds,
	}
}

// GET /data/:chain/all, requires guild member authorization
func (s *DataController) GetData(c *gin.Context) {
	chainId := c.Param("chain")
	errCode, err := auth.EnsureGuildMember(c, s.ds, chainId)
	if err != nil {
		c.JSON(errCode, gin.H{"error": err.Error()})
		return
	}
	guild, err := s.ds.State.Guild(chainId)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	messages, err := s.messagesRepo.GetAllGuildMessages(chainId)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
	}
	content := make([]string, len(messages))
	for i, message := range messages {
		content[i] = message.Content
	}
	c.JSON(200, gin.H{
		"guild": gin.H{
			"name":    guild.Name,
			"id":      guild.ID,
			"icon":    guild.Icon,
			"members": guild.MemberCount,
		},
		"messages": content,
	})
}

// GET /data/:chain, requires guild member authorization
func (s *DataController) GetDataPaginated(c *gin.Context) {
	chainId := c.Param("chain")
	pageSize, err := strconv.Atoi(c.Query("pageSize"))
	if err != nil || pageSize <= 0 {
		pageSize = 100 // default page size
	}
	page, err := strconv.Atoi(c.Query("page"))
	if err != nil || page < 1 {
		page = 1 // default to first page
	}

	guild, err := s.ds.State.Guild(chainId)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	offset := (page - 1) * pageSize

	messages, total, err := s.messagesRepo.GetGuildMessagesPage(chainId, pageSize, offset)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	content := make([]string, len(messages))
	for i, message := range messages {
		content[i] = message.Content
	}

	c.JSON(200, gin.H{
		"data": gin.H{
			"guild": gin.H{
				"name":    guild.Name,
				"id":      guild.ID,
				"icon":    guild.Icon,
				"members": guild.MemberCount,
			},
			"messages": content,
		},
		"meta": gin.H{
			"currentPage": page,
			"pageSize":    pageSize,
			"totalItems":  total,
			"totalPages":  (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}
