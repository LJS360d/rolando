package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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

func FetchUserInfo(accessToken string) (user *discordgo.User, err error) {
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
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	// Return the user info
	return user, nil
}

func FetchUserGuilds(accessToken string, withCounts bool) (st *[]discordgo.UserGuild, err error) {
	// Set up the request
	v := url.Values{}

	if withCounts {
		v.Set("with_counts", "true")
	}

	uri := "https://discord.com/api/v10/users/@me/guilds"

	if len(v) > 0 {
		uri += "?" + v.Encode()
	}

	req, err := http.NewRequest("GET", uri, nil)
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
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return nil, err
	}
	return st, nil
}
