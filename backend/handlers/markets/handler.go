package marketshandlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"socialpredict/handlers/markets/dto"
	dmarkets "socialpredict/internal/domain/markets"
	authsvc "socialpredict/internal/service/auth"

	"github.com/gorilla/mux"
)

// Service defines the interface for the markets domain service
type Service interface {
	CreateMarket(ctx context.Context, req dmarkets.MarketCreateRequest, creatorUsername string) (*dmarkets.Market, error)
	SetCustomLabels(ctx context.Context, marketID int64, yesLabel, noLabel string) error
	GetMarket(ctx context.Context, id int64) (*dmarkets.Market, error)
	ListMarkets(ctx context.Context, filters dmarkets.ListFilters) ([]*dmarkets.Market, error)
	GetMarketDetails(ctx context.Context, marketID int64) (*dmarkets.MarketOverview, error)
	SearchMarkets(ctx context.Context, query string, filters dmarkets.SearchFilters) (*dmarkets.SearchResults, error)
	ResolveMarket(ctx context.Context, marketID int64, resolution string, username string) error
	ListByStatus(ctx context.Context, status string, p dmarkets.Page) ([]*dmarkets.Market, error)
	GetMarketLeaderboard(ctx context.Context, marketID int64, p dmarkets.Page) ([]*dmarkets.LeaderboardRow, error)
	ProjectProbability(ctx context.Context, req dmarkets.ProbabilityProjectionRequest) (*dmarkets.ProbabilityProjection, error)
}

// Handler handles HTTP requests for markets
type Handler struct {
	service Service
	auth    authsvc.Authenticator
}

// NewHandler creates a new markets handler
func NewHandler(service Service, auth authsvc.Authenticator) *Handler {
	return &Handler{
		service: service,
		auth:    auth,
	}
}

// CreateMarket handles POST /markets
func (h *Handler) CreateMarket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	if h.auth == nil {
		writeInternalFailure(w)
		return
	}

	// Validate user authentication via auth service
	user, httperr := h.auth.CurrentUser(r)
	if httperr != nil {
		writeAuthFailure(w, httperr)
		return
	}

	// Parse request body
	var req dto.CreateMarketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInvalidRequest(w)
		return
	}

	// Convert DTO to domain model
	createReq := dmarkets.MarketCreateRequest{
		QuestionTitle:      req.QuestionTitle,
		Description:        req.Description,
		OutcomeType:        req.OutcomeType,
		ResolutionDateTime: req.ResolutionDateTime,
		YesLabel:           req.YesLabel,
		NoLabel:            req.NoLabel,
	}

	// Call service
	market, err := h.service.CreateMarket(r.Context(), createReq, user.Username)
	if err != nil {
		writeMarketFailure(w, err)
		return
	}

	// Convert to response DTO
	response := marketToResponse(market)

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// UpdateLabels handles PUT /markets/{id}/labels
func (h *Handler) UpdateLabels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeMethodNotAllowed(w)
		return
	}

	// Parse market ID from URL
	vars := mux.Vars(r)
	idStr := vars["id"]
	if idStr == "" {
		writeInvalidRequest(w)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeInvalidRequest(w)
		return
	}

	// Parse request body
	var req dto.UpdateLabelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInvalidRequest(w)
		return
	}

	// Call service
	if err := h.service.SetCustomLabels(r.Context(), id, req.YesLabel, req.NoLabel); err != nil {
		writeMarketFailure(w, err)
		return
	}

	// Send success response
	w.WriteHeader(http.StatusNoContent)
}

// GetMarket handles GET /markets/{id}
func (h *Handler) GetMarket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	// Parse market ID from URL
	vars := mux.Vars(r)
	idStr := vars["id"]
	if idStr == "" {
		writeInvalidRequest(w)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeInvalidRequest(w)
		return
	}

	// Call service
	market, err := h.service.GetMarket(r.Context(), id)
	if err != nil {
		writeMarketFailure(w, err)
		return
	}

	// Convert to response DTO
	response := marketToResponse(market)

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ListMarkets handles GET /markets
func (h *Handler) ListMarkets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	params, err := parseListMarketsParams(r)
	if err != nil {
		writeInvalidRequest(w)
		return
	}

	// Call service
	var markets []*dmarkets.Market
	if params.status != "" {
		page := dmarkets.Page{Limit: params.limit, Offset: params.offset}
		markets, err = h.service.ListByStatus(r.Context(), params.status, page)
	} else {
		markets, err = h.service.ListMarkets(r.Context(), params.filters)
	}
	if err != nil {
		writeMarketFailure(w, err)
		return
	}

	overviews, err := buildMarketOverviewResponses(r.Context(), h.service, markets)
	if err != nil {
		writeMarketFailure(w, err)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto.ListMarketsResponse{
		Markets: overviews,
		Total:   len(overviews),
	})
}

// SearchMarkets handles GET /markets/search
func (h *Handler) SearchMarkets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	params, err := h.parseSearchParams(r)
	if err != nil {
		writeInvalidRequest(w)
		return
	}

	// Call service
	searchResults, err := h.service.SearchMarkets(r.Context(), params.Query, params.Filters)
	if err != nil {
		writeMarketFailure(w, err)
		return
	}

	response, buildErr := h.buildSearchResponse(r, searchResults)
	if buildErr != nil {
		writeMarketFailure(w, buildErr)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ResolveMarket handles POST /markets/{id}/resolve
func (h *Handler) ResolveMarket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	// Parse market ID from URL
	vars := mux.Vars(r)
	idStr := vars["id"]
	if idStr == "" {
		writeInvalidRequest(w)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeInvalidRequest(w)
		return
	}

	if h.auth == nil {
		writeInternalFailure(w)
		return
	}

	// Get user for authorization
	user, httperr := h.auth.CurrentUser(r)
	if httperr != nil {
		writeAuthFailure(w, httperr)
		return
	}

	// Parse request body
	var req dto.ResolveMarketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInvalidRequest(w)
		return
	}

	// Call service
	if err := h.service.ResolveMarket(r.Context(), id, req.Resolution, user.Username); err != nil {
		writeMarketFailure(w, err)
		return
	}

	// Send success response
	w.WriteHeader(http.StatusNoContent)
}

// ListByStatus handles GET /markets/status/{status}
func (h *Handler) ListByStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	statusValue, err := parseStatusFromRequest(r)
	if err != nil {
		writeInvalidRequest(w)
		return
	}

	page := parsePagination(r, 100)

	markets, err := h.fetchMarketsByStatus(r.Context(), statusValue, page)
	if err != nil {
		writeMarketFailure(w, err)
		return
	}

	// Convert to response DTOs
	overviews, err := buildMarketOverviewResponses(r.Context(), h.service, markets)
	if err != nil {
		writeMarketFailure(w, err)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto.ListMarketsResponse{
		Markets: overviews,
		Total:   len(overviews),
	})
}

// GetDetails handles GET /markets/{id} with full market details
func (h *Handler) GetDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	// Parse market ID from URL
	vars := mux.Vars(r)
	idStr := vars["id"]
	if idStr == "" {
		writeInvalidRequest(w)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeInvalidRequest(w)
		return
	}

	// Call service
	details, err := h.service.GetMarketDetails(r.Context(), id)
	if err != nil {
		writeMarketFailure(w, err)
		return
	}

	response := dto.MarketDetailsResponse{
		Market:             publicMarketResponseFromDomain(details.Market),
		Creator:            creatorResponseFromSummary(details.Creator),
		ProbabilityChanges: probabilityChangesToResponse(details.ProbabilityChanges),
		NumUsers:           details.NumUsers,
		TotalVolume:        details.TotalVolume,
		MarketDust:         details.MarketDust,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// MarketLeaderboard handles GET /markets/{id}/leaderboard
func (h *Handler) MarketLeaderboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	id, err := parseMarketIDFromRequest(r)
	if err != nil {
		writeInvalidRequest(w)
		return
	}

	page := parsePagination(r, 100)

	// Call service
	leaderboard, err := h.service.GetMarketLeaderboard(r.Context(), id, page)
	if err != nil {
		writeMarketFailure(w, err)
		return
	}

	leaderRows := buildLeaderboardRows(leaderboard)

	response := dto.LeaderboardResponse{
		MarketID:    id,
		Leaderboard: leaderRows,
		Total:       len(leaderRows),
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ProjectProbability handles GET /markets/{id}/projection
func (h *Handler) ProjectProbability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	req, err := parseProjectionRequest(r)
	if err != nil {
		writeInvalidRequest(w)
		return
	}

	// Call service
	projection, err := h.service.ProjectProbability(r.Context(), req)
	if err != nil {
		writeMarketFailure(w, err)
		return
	}

	// Return response DTO
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

func parseStatusFromRequest(r *http.Request) (string, error) {
	vars := mux.Vars(r)
	if vars["status"] == "" {
		return "", errors.New("Status is required")
	}
	return normalizeStatusParam(vars["status"])
}

func parsePagination(r *http.Request, defaultLimit int) dmarkets.Page {
	limit := defaultLimit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	return dmarkets.Page{
		Limit:  limit,
		Offset: offset,
	}
}

func (h *Handler) fetchMarketsByStatus(ctx context.Context, statusValue string, page dmarkets.Page) ([]*dmarkets.Market, error) {
	if statusValue == "" {
		filters := dmarkets.ListFilters{
			Status: "",
			Limit:  page.Limit,
			Offset: page.Offset,
		}
		return h.service.ListMarkets(ctx, filters)
	}
	return h.service.ListByStatus(ctx, statusValue, page)
}

func parseMarketIDFromRequest(r *http.Request) (int64, error) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	if idStr == "" {
		idStr = vars["marketId"]
	}
	if idStr == "" {
		return 0, errors.New("Market ID is required")
	}

	return strconv.ParseInt(idStr, 10, 64)
}

func parseProjectionRequest(r *http.Request) (dmarkets.ProbabilityProjectionRequest, error) {
	marketID, err := parseMarketIDFromRequest(r)
	if err != nil {
		return dmarkets.ProbabilityProjectionRequest{}, err
	}

	amountRaw := r.URL.Query().Get("amount")
	if amountRaw == "" {
		amountRaw = mux.Vars(r)["amount"]
	}
	if amountRaw == "" {
		return dmarkets.ProbabilityProjectionRequest{}, errors.New("Amount is required")
	}

	amount, err := strconv.ParseInt(amountRaw, 10, 64)
	if err != nil {
		return dmarkets.ProbabilityProjectionRequest{}, errors.New("Invalid amount value")
	}

	outcome := r.URL.Query().Get("outcome")
	if outcome == "" {
		outcome = mux.Vars(r)["outcome"]
	}
	if outcome == "" {
		return dmarkets.ProbabilityProjectionRequest{}, errors.New("Outcome is required")
	}

	return dmarkets.ProbabilityProjectionRequest{
		MarketID: marketID,
		Amount:   amount,
		Outcome:  outcome,
	}, nil
}

func buildLeaderboardRows(leaderboard []*dmarkets.LeaderboardRow) []dto.LeaderboardRow {
	if len(leaderboard) == 0 {
		return []dto.LeaderboardRow{}
	}

	leaderRows := make([]dto.LeaderboardRow, 0, len(leaderboard))
	for _, row := range leaderboard {
		leaderRows = append(leaderRows, dto.LeaderboardRow{
			Username:       row.Username,
			Profit:         row.Profit,
			CurrentValue:   row.CurrentValue,
			TotalSpent:     row.TotalSpent,
			Position:       row.Position,
			YesSharesOwned: row.YesSharesOwned,
			NoSharesOwned:  row.NoSharesOwned,
			Rank:           row.Rank,
		})
	}
	return leaderRows
}

type searchParams struct {
	Query   string
	Filters dmarkets.SearchFilters
}

func (h *Handler) parseSearchParams(r *http.Request) (searchParams, error) {
	query := r.URL.Query().Get("query")
	if query == "" {
		query = r.URL.Query().Get("q")
	}
	if query == "" {
		return searchParams{}, errors.New("Query parameter 'query' is required")
	}

	status, err := normalizeStatusParam(r.URL.Query().Get("status"))
	if err != nil {
		return searchParams{}, err
	}

	return searchParams{
		Query: query,
		Filters: dmarkets.SearchFilters{
			Status: status,
			Limit:  parseLimit(r.URL.Query().Get("limit")),
			Offset: parseOffset(r.URL.Query().Get("offset")),
		},
	}, nil
}

func (h *Handler) buildSearchResponse(r *http.Request, searchResults *dmarkets.SearchResults) (dto.SearchResponse, error) {
	primaryOverviews, err := buildMarketOverviewResponses(r.Context(), h.service, searchResults.PrimaryResults)
	if err != nil {
		return dto.SearchResponse{}, err
	}

	fallbackOverviews, err := buildMarketOverviewResponses(r.Context(), h.service, searchResults.FallbackResults)
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
