package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"rolando/internal/config"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
)

func EnsureOwner(c *gin.Context, ds *discordgo.Session) (int, error) {
	authorization := c.Request.Header.Get("Authorization")
	if authorization == "" {
		return 401, errors.New("Unauthorized")
	}
	user, err := FetchUserInfo(authorization)
	if err != nil {
		return 500, err
	}
	if !slices.Contains(config.OwnerIDs, user.ID) {
		return 403, errors.New("Forbidden")
	}
	return 200, nil
}

func EnsureGuildMember(c *gin.Context, ds *discordgo.Session, guildId string) (int, error) {
	authorization := c.Request.Header.Get("Authorization")
	if authorization == "" {
		return 401, errors.New("Unauthorized")
	}
	user, err := FetchUserInfo(authorization)
	if err != nil {
		return 500, err
	}
	if slices.Contains(config.OwnerIDs, user.ID) {
		return 200, nil
	}
	_, err = ds.GuildMember(guildId, user.ID)
	if err != nil {
		return 404, fmt.Errorf("not a guild member or invalid guild id %v", err)
	}
	return 200, nil
}

func FetchUserInfo(accessToken string) (*DiscordUser, error) {
	// Set up the request
	req, err := http.NewRequest("GET", "https://discord.com/api/v10/users/@me", nil)
	if err != nil {
		return nil, err
	}

	// Add the Authorization header with the access token
	req.Header.Add("Authorization", "Bearer "+strings.TrimPrefix(accessToken, "Bearer "))

	// Perform the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse the response
	var user DiscordUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	// Return the user info
	return &user, nil
}

type DiscordUser struct {
	ID                   string  `json:"id"`
	Username             string  `json:"username"`
	Avatar               string  `json:"avatar"`
	Discriminator        string  `json:"discriminator"`
	PublicFlags          int     `json:"public_flags"`
	Flags                int     `json:"flags"`
	Banner               string  `json:"banner"`
	AccentColor          int     `json:"accent_color"`
	GlobalName           string  `json:"global_name"`
	AvatarDecorationData *string `json:"avatar_decoration_data"` // Can be null
	BannerColor          string  `json:"banner_color"`
	Clan                 *string `json:"clan"`          // Can be null
	PrimaryGuild         *string `json:"primary_guild"` // Can be null
	MFAEnabled           bool    `json:"mfa_enabled"`
	Locale               string  `json:"locale"`
	PremiumType          int     `json:"premium_type"`
}
