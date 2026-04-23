package failurereason

import (
	"errors"
	"net/http"
	"strings"

	"socialpredict/handlers"
	dusers "socialpredict/internal/domain/users"
	authsvc "socialpredict/internal/service/auth"
)

// FromAuthHTTPError maps auth boundary errors onto the shared public reason
// vocabulary used by migrated handlers.
func FromAuthHTTPError(err *authsvc.HTTPError) handlers.FailureReason {
	if err == nil {
		return handlers.ReasonInternalError
	}

	switch {
	case err.StatusCode >= http.StatusInternalServerError:
		return handlers.ReasonInternalError
	case err.StatusCode == http.StatusUnauthorized:
		return handlers.ReasonAuthenticationRequired
	case err.StatusCode == http.StatusForbidden:
		if strings.EqualFold(err.Message, "Password change required") {
			return handlers.ReasonPasswordChangeRequired
		}
		return handlers.ReasonAuthorizationDenied
	case err.StatusCode == http.StatusNotFound:
		return handlers.ReasonNotFound
	default:
		return handlers.ReasonInternalError
	}
}

// LooksLikeValidation reports whether the error string appears to describe a
// caller-fixable validation or sanitization failure rather than an internal
// server problem.
func LooksLikeValidation(err error) bool {
	if err == nil {
		return false
	}

	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "invalid") ||
		strings.Contains(lower, "exceeds") ||
		strings.Contains(lower, "must") ||
		strings.Contains(lower, "cannot") ||
		strings.Contains(lower, "required") ||
		strings.Contains(lower, "requirement") ||
		strings.Contains(lower, "unsupported") ||
		strings.Contains(lower, "mismatch")
}

// FromUserError maps users-domain failures onto the shared public reason
// vocabulary used by swept user/profile handlers.
func FromUserError(err error) (int, handlers.FailureReason) {
	switch {
	case err == nil:
		return http.StatusInternalServerError, handlers.ReasonInternalError
	case errors.Is(err, dusers.ErrUserNotFound):
		return http.StatusNotFound, handlers.ReasonNotFound
	case errors.Is(err, dusers.ErrInvalidUserData):
		return http.StatusBadRequest, handlers.ReasonValidationFailed
	case errors.Is(err, dusers.ErrInvalidCredentials):
		return http.StatusUnauthorized, handlers.ReasonInvalidCredentials
	case errors.Is(err, dusers.ErrUnauthorized):
		return http.StatusForbidden, handlers.ReasonAuthorizationDenied
	default:
		if LooksLikeValidation(err) {
			return http.StatusBadRequest, handlers.ReasonValidationFailed
		}
		return http.StatusInternalServerError, handlers.ReasonInternalError
	}
}
