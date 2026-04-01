package marketshandlers

import (
	"log"
	"net/http"

	dmarkets "socialpredict/internal/domain/markets"
)

// ListMarketsHandlerFactory creates an HTTP handler for listing markets with service injection
func ListMarketsHandlerFactory(svc dmarkets.ServiceInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("ListMarketsHandler: Request received")
		if r.Method != http.MethodGet {
			http.Error(w, "Method is not supported.", http.StatusMethodNotAllowed)
			return
		}

		params, parseErr := parseListMarketsParams(r)
		if parseErr != nil {
			http.Error(w, parseErr.Error(), http.StatusBadRequest)
			return
		}

		markets, err := fetchMarkets(r, svc, params)
		if err != nil {
			writeListMarketsError(w, err)
			return
		}

		response, err := buildListMarketsResponse(r.Context(), svc, markets)
		if err != nil {
			log.Printf("Error building market overviews: %v", err)
			http.Error(w, "Error fetching markets", http.StatusInternalServerError)
			return
		}

		if err := writeListMarketsResponse(w, response); err != nil {
			log.Printf("Error encoding response: %v", err)
			http.Error(w, "Error encoding response", http.StatusInternalServerError)
		}
	}
}

type listMarketsParams struct {
	status  string
	limit   int
	offset  int
	filters dmarkets.ListFilters
	page    dmarkets.Page
}

func parseListMarketsParams(r *http.Request) (listMarketsParams, error) {
	status, statusErr := normalizeStatusParam(r.URL.Query().Get("status"))
	if statusErr != nil {
		return listMarketsParams{}, statusErr
	}

	return newListMarketsParams(
		status,
		parseListLimit(r.URL.Query().Get("limit")),
		parseListOffset(r.URL.Query().Get("offset")),
	), nil
}

func parseListLimit(rawLimit string) int {
	return parseBoundedPositiveInt(rawLimit, 50, 100)
}

func parseListOffset(rawOffset string) int {
	return parseNonNegativeInt(rawOffset, 0)
}

func fetchMarkets(r *http.Request, svc dmarkets.ServiceInterface, params listMarketsParams) ([]*dmarkets.Market, error) {
	if params.status != "" {
		return svc.ListByStatus(r.Context(), params.status, params.page)
	}
	return svc.ListMarkets(r.Context(), params.filters)
}

func writeListMarketsError(w http.ResponseWriter, err error) {
	switch err {
	case dmarkets.ErrInvalidInput:
		http.Error(w, "Invalid input parameters", http.StatusBadRequest)
	case dmarkets.ErrUnauthorized:
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	default:
		log.Printf("Error fetching markets: %v", err)
		http.Error(w, "Error fetching markets", http.StatusInternalServerError)
	}
}

func newListMarketsParams(status string, limit, offset int) listMarketsParams {
	return listMarketsParams{
		status: status,
		limit:  limit,
		offset: offset,
		filters: dmarkets.ListFilters{
			Status: status,
			Limit:  limit,
			Offset: offset,
		},
		page: dmarkets.Page{
			Limit:  limit,
			Offset: offset,
		},
	}
}

func writeListMarketsResponse(w http.ResponseWriter, response interface{}) error {
	return writeJSONResponse(w, http.StatusOK, response)
}
