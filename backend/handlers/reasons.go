package handlers

const (
	ReasonMethodNotAllowed FailureReason = "METHOD_NOT_ALLOWED"
	ReasonInvalidRequest   FailureReason = "INVALID_REQUEST"

	// Canonical route-visible reasons for endpoints migrated onto the shared
	// API reason vocabulary.
	ReasonValidationFailed       FailureReason = "VALIDATION_FAILED"
	ReasonAuthenticationRequired FailureReason = "AUTHENTICATION_REQUIRED"
	ReasonAuthorizationDenied    FailureReason = "AUTHORIZATION_DENIED"
	ReasonPasswordChangeRequired FailureReason = "PASSWORD_CHANGE_REQUIRED"
	ReasonInvalidCredentials     FailureReason = "INVALID_CREDENTIALS"
	ReasonNotFound               FailureReason = "NOT_FOUND"
	ReasonMarketClosed           FailureReason = "MARKET_CLOSED"
	ReasonInsufficientBalance    FailureReason = "INSUFFICIENT_BALANCE"
	ReasonInvalidOutcome         FailureReason = "INVALID_OUTCOME"
	ReasonNoPosition             FailureReason = "NO_POSITION"
	ReasonInsufficientShares     FailureReason = "INSUFFICIENT_SHARES"
	ReasonDustCapExceeded        FailureReason = "DUST_CAP_EXCEEDED"
	ReasonInternalError          FailureReason = "INTERNAL_ERROR"

	// Legacy synonyms retained while older handlers are swept onto the shared
	// vocabulary.
	ReasonInvalidToken   FailureReason = "INVALID_TOKEN"
	ReasonUserNotFound   FailureReason = "USER_NOT_FOUND"
	ReasonMarketNotFound FailureReason = "MARKET_NOT_FOUND"
)

func CanonicalFailureReasons() []FailureReason {
	return []FailureReason{
		ReasonMethodNotAllowed,
		ReasonInvalidRequest,
		ReasonValidationFailed,
		ReasonAuthenticationRequired,
		ReasonAuthorizationDenied,
		ReasonPasswordChangeRequired,
		ReasonInvalidCredentials,
		ReasonNotFound,
		ReasonMarketClosed,
		ReasonInsufficientBalance,
		ReasonInvalidOutcome,
		ReasonNoPosition,
		ReasonInsufficientShares,
		ReasonDustCapExceeded,
		ReasonInternalError,
	}
}

func LegacyFailureReasons() []FailureReason {
	return []FailureReason{
		ReasonInvalidToken,
		ReasonUserNotFound,
		ReasonMarketNotFound,
	}
}

func AllFailureReasons() []FailureReason {
	reasons := CanonicalFailureReasons()
	return append(reasons, LegacyFailureReasons()...)
}
