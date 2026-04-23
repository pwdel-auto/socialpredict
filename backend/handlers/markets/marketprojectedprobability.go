package marketshandlers

import (
	"encoding/json"
	"net/http"

	"socialpredict/handlers/markets/dto"
	dmarkets "socialpredict/internal/domain/markets"
)

// ProjectNewProbabilityHandler handles the projection of a new probability based on a new bet.
func ProjectNewProbabilityHandler(svc dmarkets.ServiceInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}

		req, err := parseProjectionRequest(r)
		if err != nil {
			writeInvalidRequest(w)
			return
		}

		// 3. Call domain service
		projection, err := svc.ProjectProbability(r.Context(), req)
		if err != nil {
			writeMarketFailure(w, err)
			return
		}

		// 5. Return response DTO
		response := dto.ProbabilityProjectionResponse{
			MarketID:             req.MarketID,
			CurrentProbability:   projection.CurrentProbability,
			ProjectedProbability: projection.ProjectedProbability,
			Amount:               req.Amount,
			Outcome:              req.Outcome,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
