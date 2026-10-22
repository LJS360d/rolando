package common

import "context"

type Jackbox interface {
	GenerateLine(ctx context.Context, guildID string, maxWords int) (string, error)
}
