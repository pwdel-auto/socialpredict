package positions

import dmarkets "socialpredict/internal/domain/markets"

// UserPositionResponse defines the JSON shape returned to clients.
type UserPositionResponse struct {
	Username         string `json:"username"`
	MarketID         int64  `json:"marketId"`
	YesSharesOwned   int64  `json:"yesSharesOwned"`
	NoSharesOwned    int64  `json:"noSharesOwned"`
	Value            int64  `json:"value"`
	TotalSpent       int64  `json:"totalSpent"`
	TotalSpentInPlay int64  `json:"totalSpentInPlay"`
	IsResolved       bool   `json:"isResolved"`
	ResolutionResult string `json:"resolutionResult"`
}

// NewUserPositionResponse converts the markets-domain position into the shared
// user-position transport shape used by both public market-position and
// authenticated user-position routes.
func NewUserPositionResponse(pos *dmarkets.UserPosition) UserPositionResponse {
	if pos == nil {
		return UserPositionResponse{}
	}

	return UserPositionResponse{
		Username:         pos.Username,
		MarketID:         pos.MarketID,
		YesSharesOwned:   pos.YesSharesOwned,
		NoSharesOwned:    pos.NoSharesOwned,
		Value:            pos.Value,
		TotalSpent:       pos.TotalSpent,
		TotalSpentInPlay: pos.TotalSpentInPlay,
		IsResolved:       pos.IsResolved,
		ResolutionResult: pos.ResolutionResult,
	}
}
