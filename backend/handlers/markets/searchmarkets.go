package marketshandlers

import (
	"context"
	"errors"
	"log"
	"net/http"

	"socialpredict/handlers/markets/dto"
	dmarkets "socialpredict/internal/domain/markets"
)

// SearchMarketsHandler handles HTTP requests for searching markets - HTTP-only with service injection
func SearchMarketsHandler(svc dmarkets.ServiceInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("SearchMarketsHandler: Request received")
		if r.Method != http.MethodGet {
			http.Error(w, "Method is not supported.", http.StatusMethodNotAllowed)
			return
		}

		params, clientErr := parseSearchRequest(r)
		if clientErr != nil {
			http.Error(w, clientErr.message, clientErr.statusCode)
			return
		}

		// ms, err := h.service.SearchMarkets(r.Context(), q, f)
		searchResults, err := svc.SearchMarkets(r.Context(), params.query, params.filters)
		if err != nil {
			writeSearchMarketsError(w, err)
			return
		}

		if err := writeSearchResponse(w, r.Context(), svc, searchResults); err != nil {
			log.Printf("Error encoding search response: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

type searchRequestParams struct {
	query   string
	filters dmarkets.SearchFilters
}

func parseSearchRequest(r *http.Request) (searchRequestParams, *httpError) {
	query, queryErr := extractQuery(r)
	if queryErr != nil {
		return searchRequestParams{}, queryErr
	}

	status, statusErr := normalizeStatusParam(r.URL.Query().Get("status"))
	if statusErr != nil {
		return searchRequestParams{}, newHTTPError(http.StatusBadRequest, statusErr.Error())
	}

	filters := dmarkets.SearchFilters{
		Status: status,
		Limit:  parseLimit(r.URL.Query().Get("limit")),
		Offset: parseOffset(r.URL.Query().Get("offset")),
	}

	return searchRequestParams{
		query:   query,
		filters: filters,
	}, nil
}

func extractQuery(r *http.Request) (string, *httpError) {
	query := firstQueryValue(r.URL.Query(), "query", "q")
	if query == "" {
		return "", newHTTPError(http.StatusBadRequest, "Query parameter 'query' is required")
	}

	return sanitizeQuery(query)
}

func sanitizeQuery(query string) (string, *httpError) {
	sanitizedQuery, err := sanitizeMarketQuery(query)
	if err != nil {
		log.Printf("SearchMarketsHandler: Sanitization failed for query '%s': %v", query, err)
		return "", err
	}
	return sanitizedQuery, nil
}

func parseLimit(rawLimit string) int {
	return parseBoundedPositiveInt(rawLimit, 20, 50)
}

func parseOffset(rawOffset string) int {
	return parseNonNegativeInt(rawOffset, 0)
}

func buildSearchResponse(ctx context.Context, svc dmarkets.ServiceInterface, searchResults *dmarkets.SearchResults) (dto.SearchResponse, error) {
	primaryOverviews, err := buildSearchResultOverviews(ctx, svc, searchResults.PrimaryResults, "primary")
	if err != nil {
		return dto.SearchResponse{}, err
	}

	fallbackOverviews, err := buildSearchResultOverviews(ctx, svc, searchResults.FallbackResults, "fallback")
	if err != nil {
		return dto.SearchResponse{}, err
	}

	return dto.SearchResponse{
		PrimaryResults:  primaryOverviews,
		FallbackResults: fallbackOverviews,
		Query:           searchResults.Query,
		PrimaryStatus:   searchResults.PrimaryStatus,
		PrimaryCount:    searchResults.PrimaryCount,
		FallbackCount:   searchResults.FallbackCount,
		TotalCount:      searchResults.TotalCount,
		FallbackUsed:    searchResults.FallbackUsed,
	}, nil
}

func buildSearchResultOverviews(ctx context.Context, svc dmarkets.ServiceInterface, markets []*dmarkets.Market, resultType string) ([]*dto.MarketOverviewResponse, error) {
	overviews, err := buildMarketOverviewResponses(ctx, svc, markets)
	if err != nil {
		log.Printf("Error building %s results: %v", resultType, err)
		return nil, errors.New("Error building " + resultType + " results")
	}

	return overviews, nil
}

func writeSearchResponse(w http.ResponseWriter, ctx context.Context, svc dmarkets.ServiceInterface, searchResults *dmarkets.SearchResults) error {
	response, err := buildSearchResponse(ctx, svc, searchResults)
	if err != nil {
		return err
	}

	return writeJSONResponse(w, http.StatusOK, response)
}

func writeSearchMarketsError(w http.ResponseWriter, err error) {
	switch err {
	case dmarkets.ErrInvalidInput:
		http.Error(w, "Invalid search parameters", http.StatusBadRequest)
	default:
		log.Printf("Error searching markets: %v", err)
		http.Error(w, "Error searching markets", http.StatusInternalServerError)
	}
}
