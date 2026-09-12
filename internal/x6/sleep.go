package x6

import protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"

const (
	NormalSleepMinimumMinutes = protocol.NormalSleepMinimumMinutes
	NormalSleepMaximumMinutes = protocol.NormalSleepMaximumMinutes
)

func ValidateNormalSleepMinutes(minutes float64) error { return protocol.ValidateNormalSleepMinutes(minutes) }
