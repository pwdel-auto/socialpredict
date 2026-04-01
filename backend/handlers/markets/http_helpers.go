package marketshandlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"socialpredict/handlers/markets/dto"
	dmarkets "socialpredict/internal/domain/markets"
	"socialpredict/security"
)

type httpError struct {
	message    string
	statusCode int
}

func (e *httpError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func newHTTPError(statusCode int, message string) *httpError {
	return &httpError{
		message:    message,
		statusCode: statusCode,
	}
}

func firstQueryValue(values url.Values, keys ...string) string {
	for _, key := range keys {
		if value := values.Get(key); value != "" {
			return value
		}
	}
	return ""
}

func sanitizeMarketQuery(query string) (string, *httpError) {
	sanitizedQuery, err := security.NewSanitizer().SanitizeMarketTitle(query)
	if err != nil {
		return "", newHTTPError(http.StatusBadRequest, "Invalid search query: "+err.Error())
	}
	if len(sanitizedQuery) > 100 {
		return "", newHTTPError(http.StatusBadRequest, "Query too long (max 100 characters)")
	}
	return sanitizedQuery, nil
}

func parseBoundedPositiveInt(raw string, defaultValue, maxValue int) int {
	if raw == "" {
		return defaultValue
	}

	parsedValue, err := strconv.Atoi(raw)
	if err != nil || parsedValue <= 0 || parsedValue > maxValue {
		return defaultValue
	}
	return parsedValue
}

func parseNonNegativeInt(raw string, defaultValue int) int {
	if raw == "" {
		return defaultValue
	}

	parsedValue, err := strconv.Atoi(raw)
	if err != nil || parsedValue < 0 {
		return defaultValue
	}
	return parsedValue
}

func buildListMarketsResponse(ctx context.Context, svc dmarkets.ServiceInterface, markets []*dmarkets.Market) (dto.ListMarketsResponse, error) {
	overviews, err := buildMarketOverviewResponses(ctx, svc, markets)
	if err != nil {
		return dto.ListMarketsResponse{}, err
	}

	return dto.ListMarketsResponse{
		Markets: overviews,
		Total:   len(overviews),
	}, nil
}

func writeJSONResponse(w http.ResponseWriter, statusCode int, payload interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(payload)
}
