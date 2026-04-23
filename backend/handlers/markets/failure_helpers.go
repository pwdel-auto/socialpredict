package marketshandlers

import (
	"errors"
	"net/http"

	"socialpredict/handlers"
	"socialpredict/handlers/failurereason"
	dmarkets "socialpredict/internal/domain/markets"
	authsvc "socialpredict/internal/service/auth"
)

func writeMethodNotAllowed(w http.ResponseWriter) {
	_ = handlers.WriteFailure(w, http.StatusMethodNotAllowed, handlers.ReasonMethodNotAllowed)
}

func writeInvalidRequest(w http.ResponseWriter) {
	_ = handlers.WriteFailure(w, http.StatusBadRequest, handlers.ReasonInvalidRequest)
}

func writeInternalFailure(w http.ResponseWriter) {
	_ = handlers.WriteFailure(w, http.StatusInternalServerError, handlers.ReasonInternalError)
}

func writeAuthFailure(w http.ResponseWriter, err *authsvc.HTTPError) {
	if err == nil {
		writeInternalFailure(w)
		return
	}

	statusCode := err.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusInternalServerError
	}

	_ = handlers.WriteFailure(w, statusCode, failurereason.FromAuthHTTPError(err))
}

func writeMarketFailure(w http.ResponseWriter, err error) {
	statusCode, reason := marketFailure(err)
	_ = handlers.WriteFailure(w, statusCode, reason)
}

func marketFailure(err error) (int, handlers.FailureReason) {
	switch {
	case errors.Is(err, dmarkets.ErrMarketNotFound), errors.Is(err, dmarkets.ErrUserNotFound):
		return http.StatusNotFound, handlers.ReasonNotFound
	case errors.Is(err, dmarkets.ErrInvalidQuestionTitle),
		errors.Is(err, dmarkets.ErrInvalidQuestionLength),
		errors.Is(err, dmarkets.ErrInvalidDescriptionLength),
		errors.Is(err, dmarkets.ErrInvalidLabel),
		errors.Is(err, dmarkets.ErrInvalidResolutionTime):
		return http.StatusBadRequest, handlers.ReasonValidationFailed
	case errors.Is(err, dmarkets.ErrInsufficientBalance):
		return http.StatusBadRequest, handlers.ReasonInsufficientBalance
	case errors.Is(err, dmarkets.ErrUnauthorized):
		return http.StatusForbidden, handlers.ReasonAuthorizationDenied
	case errors.Is(err, dmarkets.ErrInvalidState):
		return http.StatusConflict, handlers.ReasonValidationFailed
	case errors.Is(err, dmarkets.ErrInvalidInput):
		return http.StatusBadRequest, handlers.ReasonValidationFailed
	default:
		return http.StatusInternalServerError, handlers.ReasonInternalError
	}
}
