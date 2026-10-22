package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"
)

var (
	Token                string
	Intents              gateway.Intents
	OwnerIDs             []string
	Version              string
	InviteUrl            string
	SupportURL           string
	Env                  string
	DataDir              string
	ShardCount           int
	NodeCount            int
	NodeIndex            int
	NodeAddr             string
	NodeAPIAddress       string
	NodeSecret           string
	SessionSecret        string
	OAuthClientSecret    string
	SiteURL              string
	ChainIdleTTL         time.Duration
	SnapshotInterval     time.Duration
	ServerAddress        string
	StartupTime          time.Time
	RunHttpServer        bool
	ForceCommandRefresh  bool
	PaywallsEnabled      bool
	VoiceChatFeaturesSKU snowflake.ID
	PremiumsPageLink     string
)

func init() {
	log.Println("Initializing config...")
	Env = os.Getenv("GO_ENV")
	if Env == "" {
		log.Println("GO_ENV not set in the environment")
		Env = "production"
	}

	// Assign the environment variables to package-level variables
	Token = os.Getenv("TOKEN")
	SupportURL = os.Getenv("SUPPORT_URL")
	InviteUrl = os.Getenv("INVITE_URL")
	if InviteUrl == "" {
		log.Println("INVITE_URL not set in the environment")
	}
	ownerIDsStr := os.Getenv("OWNER_IDS")
	if ownerIDsStr == "" {
		log.Println("OWNER_IDS not set in the environment")
	} else {
		OwnerIDs = strings.Split(ownerIDsStr, ",")
	}
	DataDir = envOr("DATA_DIR", "data")
	ShardCount = envInt("SHARD_COUNT", 16)
	NodeCount = envInt("NODE_COUNT", 1)
	NodeIndex = envInt("NODE_INDEX", 0)
	NodeAddr = envOr("NODE_ADDR", "http://rolando-%d:9000")
	NodeAPIAddress = envOr("NODE_API_ADDRESS", "0.0.0.0:9000")
	NodeSecret = os.Getenv("NODE_SECRET")
	if NodeCount > 1 && NodeSecret == "" {
		log.Fatalf("NODE_SECRET is required when NODE_COUNT > 1")
	}
	SessionSecret = os.Getenv("SESSION_SECRET")
	OAuthClientSecret = os.Getenv("OAUTH_CLIENT_SECRET")
	SiteURL = strings.TrimSuffix(envOr("SITE_URL", "http://localhost:4321"), "/")
	ChainIdleTTL = envDuration("CHAIN_IDLE_TTL", 30*time.Minute)
	SnapshotInterval = envDuration("SNAPSHOT_INTERVAL", 10*time.Minute)
	ServerAddress = os.Getenv("SERVER_ADDRESS")
	if ServerAddress == "" {
		ServerAddress = "127.0.0.1:8080"
	}
	ForceCommandRefresh = os.Getenv("FORCE_COMMAND_REFRESH") == "true" || os.Getenv("FORCE_COMMAND_REFRESH") == "1" || os.Getenv("FORCE_COMMAND_REFRESH") == ""
	RunHttpServer = os.Getenv("RUN_HTTP_SERVER") == "true" || os.Getenv("RUN_HTTP_SERVER") == "1" || os.Getenv("RUN_HTTP_SERVER") == ""
	StartupTime = time.Now()

	PaywallsEnabled = os.Getenv("PAYWALLS_ENABLED") == "true" || os.Getenv("PAYWALLS_ENABLED") == "1" || os.Getenv("PAYWALLS_ENABLED") == ""
	voiceChatFeaturesSKUStr := os.Getenv("VOICE_CHAT_FEATURES_SKU_ID")
	if voiceChatFeaturesSKUStr != "" {
		var err error
		VoiceChatFeaturesSKU, err = snowflake.Parse(voiceChatFeaturesSKUStr)
		if err != nil {
			log.Printf("VOICE_CHAT_FEATURES_SKU_ID '%s' is invalid snowflake\n", voiceChatFeaturesSKUStr)
		}
	}

	PremiumsPageLink = os.Getenv("PREMIUMS_PAGE_LINK")
	Intents = (gateway.IntentDirectMessageReactions |
		gateway.IntentDirectMessageTyping |
		gateway.IntentDirectMessages |
		gateway.IntentGuildVoiceStates |
		// gateway.IntentAutoModerationConfiguration |
		// gateway.IntentAutoModerationExecution |
		// gateway.IntentDirectMessageReactions |
		// gateway.IntentGuildEmojisAndStickers |
		gateway.IntentGuildIntegrations |
		gateway.IntentGuildInvites |
		// gateway.IntentsGuildMembers |
		gateway.IntentGuildMessageReactions |
		gateway.IntentGuildMessageTyping |
		gateway.IntentGuildMessages |
		// gateway.IntentGuildModeration |
		// gateway.IntentGuildPresences |
		// gateway.IntentGuildScheduledEvents |
		// gateway.IntentGuildVoiceStates |
		gateway.IntentGuildWebhooks |
		gateway.IntentGuilds |
		gateway.IntentMessageContent)

	log.Println("Config initialized")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return def
	}
	return v
}

func envDuration(key string, def time.Duration) time.Duration {
	v, err := time.ParseDuration(os.Getenv(key))
	if err != nil {
		return def
	}
	return v
}
