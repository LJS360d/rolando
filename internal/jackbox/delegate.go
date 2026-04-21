package jackbox

import (
	"context"

	"rolando/internal/jackbox/common"
)

type GameModule = common.GameModule

func IsGameSupported(appTag string) bool {
	return common.IsSupported(appTag)
}

func SupportedGameTags() []string {
	return common.SupportedTags()
}

func NewGameModuleForSession(ctx context.Context, appTag, guildLabel, guildID string, sess *RoomSession, jackboxService common.Jackbox) (GameModule, error) {
	return common.Default().NewModule(ctx, appTag, common.Session{
		GuildLabel:  guildLabel,
		GuildID:     guildID,
		AppTag:      appTag,
		RoomCode:    sess.RoomCode,
		UserID:      sess.UserID,
		DisplayName: sess.Name,
		Conn:        sess.Conn,
	}, jackboxService)
}
