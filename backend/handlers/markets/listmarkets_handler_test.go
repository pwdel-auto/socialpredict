package marketshandlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"socialpredict/handlers/markets/dto"
	dmarkets "socialpredict/internal/domain/markets"
)

type listMarketsServiceMock struct {
	listMarketsResult  []*dmarkets.Market
	listMarketsErr     error
	listByStatusResult []*dmarkets.Market
	listByStatusErr    error
	overviews          map[int64]*dmarkets.MarketOverview

	capturedFilters *dmarkets.ListFilters
	capturedStatus  string
	capturedPage    *dmarkets.Page
}

func (m *listMarketsServiceMock) CreateMarket(ctx context.Context, req dmarkets.MarketCreateRequest, creatorUsername string) (*dmarkets.Market, error) {
	return nil, nil
}

func (m *listMarketsServiceMock) SetCustomLabels(ctx context.Context, marketID int64, yesLabel, noLabel string) error {
	return nil
}

func (m *listMarketsServiceMock) GetMarket(ctx context.Context, id int64) (*dmarkets.Market, error) {
	return nil, nil
}

func (m *listMarketsServiceMock) ListMarkets(ctx context.Context, filters dmarkets.ListFilters) ([]*dmarkets.Market, error) {
	m.capturedFilters = &filters
	return m.listMarketsResult, m.listMarketsErr
}

func (m *listMarketsServiceMock) SearchMarkets(ctx context.Context, query string, filters dmarkets.SearchFilters) (*dmarkets.SearchResults, error) {
	return nil, nil
}

func (m *listMarketsServiceMock) ResolveMarket(ctx context.Context, marketID int64, resolution string, username string) error {
	return nil
}

func (m *listMarketsServiceMock) ListByStatus(ctx context.Context, status string, p dmarkets.Page) ([]*dmarkets.Market, error) {
	m.capturedStatus = status
	m.capturedPage = &p
	return m.listByStatusResult, m.listByStatusErr
}

func (m *listMarketsServiceMock) GetMarketLeaderboard(ctx context.Context, marketID int64, p dmarkets.Page) ([]*dmarkets.LeaderboardRow, error) {
	return nil, nil
}

func (m *listMarketsServiceMock) ProjectProbability(ctx context.Context, req dmarkets.ProbabilityProjectionRequest) (*dmarkets.ProbabilityProjection, error) {
	return nil, nil
}

func (m *listMarketsServiceMock) GetMarketDetails(ctx context.Context, marketID int64) (*dmarkets.MarketOverview, error) {
	if overview, ok := m.overviews[marketID]; ok {
		return overview, nil
	}
	return nil, errors.New("overview not found")
}

func (m *listMarketsServiceMock) GetMarketBets(ctx context.Context, marketID int64) ([]*dmarkets.BetDisplayInfo, error) {
	return nil, nil
}

func (m *listMarketsServiceMock) GetMarketPositions(ctx context.Context, marketID int64) (dmarkets.MarketPositions, error) {
	return nil, nil
}

func (m *listMarketsServiceMock) GetUserPositionInMarket(ctx context.Context, marketID int64, username string) (*dmarkets.UserPosition, error) {
	return nil, nil
}

func (m *listMarketsServiceMock) CalculateMarketVolume(ctx context.Context, marketID int64) (int64, error) {
	return 0, nil
}

func (m *listMarketsServiceMock) GetPublicMarket(ctx context.Context, marketID int64) (*dmarkets.PublicMarket, error) {
	return &dmarkets.PublicMarket{ID: marketID}, nil
}

func TestListMarketsHandlerFactorySuccess(t *testing.T) {
	mockSvc := &listMarketsServiceMock{
		listMarketsResult: []*dmarkets.Market{
			{ID: 11, QuestionTitle: "Will BTC close green?"},
		},
		overviews: map[int64]*dmarkets.MarketOverview{
			11: {
				Market: &dmarkets.Market{ID: 11, QuestionTitle: "Will BTC close green?"},
				Creator: &dmarkets.CreatorSummary{
					Username: "maker",
				},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v0/markets?limit=15&offset=4", nil)
	res := httptest.NewRecorder()

	ListMarketsHandlerFactory(mockSvc)(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}

	if mockSvc.capturedFilters == nil {
		t.Fatal("expected ListMarkets to be called")
	}

	if mockSvc.capturedFilters.Status != "" || mockSvc.capturedFilters.Limit != 15 || mockSvc.capturedFilters.Offset != 4 {
		t.Fatalf("unexpected filters captured: %+v", *mockSvc.capturedFilters)
	}

	var resp dto.ListMarketsResponse
	if err := json.Unmarshal(res.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Total != 1 || len(resp.Markets) != 1 || resp.Markets[0].Market.ID != 11 {
		t.Fatalf("unexpected list response: %+v", resp)
	}
}

func TestListMarketsHandlerFactoryStatusRouting(t *testing.T) {
	mockSvc := &listMarketsServiceMock{
		listByStatusResult: []*dmarkets.Market{
			{ID: 22, QuestionTitle: "Resolved market", Status: "resolved"},
		},
		overviews: map[int64]*dmarkets.MarketOverview{
			22: {
				Market: &dmarkets.Market{ID: 22, QuestionTitle: "Resolved market", Status: "resolved"},
				Creator: &dmarkets.CreatorSummary{
					Username: "maker",
				},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v0/markets?status=resolved&limit=999&offset=-3", nil)
	res := httptest.NewRecorder()

	ListMarketsHandlerFactory(mockSvc)(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}

	if mockSvc.capturedPage == nil {
		t.Fatal("expected ListByStatus to be called")
	}

	if mockSvc.capturedStatus != "resolved" {
		t.Fatalf("expected status filter 'resolved', got %q", mockSvc.capturedStatus)
	}

	if mockSvc.capturedPage.Limit != 50 || mockSvc.capturedPage.Offset != 0 {
		t.Fatalf("unexpected page captured: %+v", *mockSvc.capturedPage)
	}
}

func TestListMarketsHandlerFactoryValidationAndErrors(t *testing.T) {
	t.Run("rejects invalid status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v0/markets?status=invalid", nil)
		res := httptest.NewRecorder()

		ListMarketsHandlerFactory(&listMarketsServiceMock{})(res, req)

		if res.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
		}
	})

	t.Run("maps service errors", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v0/markets", nil)
		res := httptest.NewRecorder()

		ListMarketsHandlerFactory(&listMarketsServiceMock{
			listMarketsErr: dmarkets.ErrUnauthorized,
		})(res, req)

		if res.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, res.Code)
		}
	})
}

func TestParseListMarketsParamsDefaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v0/markets?limit=bad&offset=-5&status=all", nil)

	params, err := parseListMarketsParams(req)
	if err != nil {
		t.Fatalf("parseListMarketsParams returned error: %v", err)
	}

	if params.status != "" {
		t.Fatalf("expected empty status, got %q", params.status)
	}

	if params.limit != 50 || params.offset != 0 {
		t.Fatalf("unexpected pagination values: %+v", params)
	}

	if params.filters.Limit != 50 || params.filters.Offset != 0 {
		t.Fatalf("unexpected filters: %+v", params.filters)
	}

	if params.page.Limit != 50 || params.page.Offset != 0 {
		t.Fatalf("unexpected page: %+v", params.page)
	}
}
