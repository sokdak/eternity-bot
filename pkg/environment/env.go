package environment

import "os"

var (
	DiscordAPIKey  = lookupEnv("DISCORD_API_KEY", "fake")
	DiscordGuildID = lookupEnv("DISCORD_GUILD_ID", "fake")

	DiscordGuildInfoChannelID        = lookupEnv("DISCORD_GI_CHANNEL_ID", "fake")
	DiscordGuildInfoByRoleMessageID  = lookupEnv("DISCORD_GIBR_MESSAGE_ID", "fake")
	DiscordGuildInfoByLevelMessageID = lookupEnv("DISCORD_GIBL_MESSAGE_ID", "fake")
	DiscordCounselChannelID          = lookupEnv("DISCORD_COUNSEL_CHANNEL_ID", "fake")

	NotionBotAPIKey   = lookupEnv("NOTION_BOT_API_KEY", "fake")
	NotionCounselDBID = lookupEnv("NOTION_COUNSEL_DB_ID", "fake")
)

func lookupEnv(key string, def string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return def
}
