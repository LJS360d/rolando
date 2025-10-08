package auth

import (
	"rolando/internal/config"
	"slices"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
)

type AuthController struct {
	ds *discordgo.Session
}

func NewController(ds *discordgo.Session) *AuthController {
	return &AuthController{
		ds: ds,
	}
}

// GET /auth/@me, public
func (s *AuthController) GetUser(c *gin.Context) {
	token := c.Request.Header.Get("Authorization")
	user, err := FetchUserInfo(token)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	var guilds []string
	userGuilds, err := FetchUserGuilds(token, false)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	for _, guild := range *userGuilds {
		guilds = append(guilds, guild.ID)
	}

	c.JSON(200, gin.H{
		"user":     user,
		"is_owner": slices.Contains(config.OwnerIDs, user.ID),
		"guilds":   guilds,
	})
}
