package marketshandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"socialpredict/handlers"
	"socialpredict/handlers/markets/dto"
	dmarkets "socialpredict/internal/domain/markets"
	dusers "socialpredict/internal/domain/users"
	authsvc "socialpredict/internal/service/auth"

	"github.com/gorilla/mux"
)

type handlerServiceStub struct {
	createMarketFn       func(ctx context.Context, req dmarkets.MarketCreateRequest, creatorUsername string) (*dmarkets.Market, error)
	listMarketsFn        func(ctx context.Context, filters dmarkets.ListFilters) ([]*dmarkets.Market, error)
	searchMarketsFn      func(ctx context.Context, query string, filters dmarkets.SearchFilters) (*dmarkets.SearchResults, error)
	resolveMarketFn      func(ctx context.Context, marketID int64, resolution string, username string) error
	getMarketDetailsFn   func(ctx context.Context, marketID int64) (*dmarkets.MarketOverview, error)
	projectProbabilityFn func(ctx context.Context, req dmarkets.ProbabilityProjectionRequest) (*dmarkets.ProbabilityProjection, error)
}

func (s *handlerServiceStub) CreateMarket(ctx context.Context, req dmarkets.MarketCreateRequest, creatorUsername string) (*dmarkets.Market, error) {
	if s.createMarketFn != nil {
		return s.createMarketFn(ctx, req, creatorUsername)
	}
	return nil, nil
}

func (s *handlerServiceStub) SetCustomLabels(context.Context, int64, string, string) error {
	return nil
}

func (s *handlerServiceStub) GetMarket(context.Context, int64) (*dmarkets.Market, error) {
	return nil, nil
}

func (s *handlerServiceStub) ListMarkets(ctx context.Context, filters dmarkets.ListFilters) ([]*dmarkets.Market, error) {
	if s.listMarketsFn != nil {
		return s.listMarketsFn(ctx, filters)
	}
	return []*dmarkets.Market{}, nil
}

func (s *handlerServiceStub) SearchMarkets(ctx context.Context, query string, filters dmarkets.SearchFilters) (*dmarkets.SearchResults, error) {
	if s.searchMarketsFn != nil {
		return s.searchMarketsFn(ctx, query, filters)
	}
	return &dmarkets.SearchResults{}, nil
}

func (s *handlerServiceStub) ResolveMarket(ctx context.Context, marketID int64, resolution string, username string) error {
	if s.resolveMarketFn != nil {
		return s.resolveMarketFn(ctx, marketID, resolution, username)
	}
	return nil
}

func (s *handlerServiceStub) ListByStatus(context.Context, string, dmarkets.Page) ([]*dmarkets.Market, error) {
	return []*dmarkets.Market{}, nil
}

func (s *handlerServiceStub) GetMarketLeaderboard(context.Context, int64, dmarkets.Page) ([]*dmarkets.LeaderboardRow, error) {
	return []*dmarkets.LeaderboardRow{}, nil
}

func (s *handlerServiceStub) ProjectProbability(ctx context.Context, req dmarkets.ProbabilityProjectionRequest) (*dmarkets.ProbabilityProjection, error) {
	if s.projectProbabilityFn != nil {
		return s.projectProbabilityFn(ctx, req)
	}
	return nil, nil
}

func (s *handlerServiceStub) GetMarketDetails(ctx context.Context, marketID int64) (*dmarkets.MarketOverview, error) {
	if s.getMarketDetailsFn != nil {
		return s.getMarketDetailsFn(ctx, marketID)
	}
	return nil, nil
}

type authStub struct {
	currentUserFn func(r *http.Request) (*dusers.User, *authsvc.HTTPError)
}

func (a authStub) CurrentUser(r *http.Request) (*dusers.User, *authsvc.HTTPError) {
	if a.currentUserFn != nil {
		return a.currentUserFn(r)
	}
	return nil, nil
}

func (a authStub) RequireUser(r *http.Request) (*dusers.User, *authsvc.HTTPError) {
	return a.CurrentUser(r)
}

func (a authStub) RequireAdmin(r *http.Request) (*dusers.User, *authsvc.HTTPError) {
	return a.CurrentUser(r)
}

func TestHandlerCreateMarketFailureEnvelope(t *testing.T) {
	body := bytes.NewBufferString(`{"questionTitle":"Question?","outcomeType":"BINARY","resolutionDateTime":"2035-01-02T03:04:05Z"}`)

	t.Run("authentication required", func(t *testing.T) {
		handler := NewHandler(&handlerServiceStub{}, authStub{
			currentUserFn: func(*http.Request) (*dusers.User, *authsvc.HTTPError) {
				return nil, &authsvc.HTTPError{StatusCode: http.StatusUnauthorized, Message: "missing token"}
			},
		})

		req := httptest.NewRequest(http.MethodPost, "/v0/markets", bytes.NewReader(body.Bytes()))
		rr := httptest.NewRecorder()

		handler.CreateMarket(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}

		var resp handlers.FailureEnvelope
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode failure envelope: %v", err)
		}
		if resp.Reason != string(handlers.ReasonAuthenticationRequired) {
			t.Fatalf("expected %q, got %q", handlers.ReasonAuthenticationRequired, resp.Reason)
		}
	})

	t.Run("insufficient balance", func(t *testing.T) {
		handler := NewHandler(&handlerServiceStub{
			createMarketFn: func(context.Context, dmarkets.MarketCreateRequest, string) (*dmarkets.Market, error) {
				return nil, dmarkets.ErrInsufficientBalance
			},
		}, authStub{
			currentUserFn: func(*http.Request) (*dusers.User, *authsvc.HTTPError) {
				return &dusers.User{Username: "creator"}, nil
			},
		})

		req := httptest.NewRequest(http.MethodPost, "/v0/markets", bytes.NewReader(body.Bytes()))
		rr := httptest.NewRecorder()

		handler.CreateMarket(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}

		var resp handlers.FailureEnvelope
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode failure envelope: %v", err)
		}
		if resp.Reason != string(handlers.ReasonInsufficientBalance) {
			t.Fatalf("expected %q, got %q", handlers.ReasonInsufficientBalance, resp.Reason)
		}
	})
}

func TestHandlerListMarketsUsesCreatedByFilter(t *testing.T) {
	var captured dmarkets.ListFilters

	handler := NewHandler(&handlerServiceStub{
		listMarketsFn: func(ctx context.Context, filters dmarkets.ListFilters) ([]*dmarkets.Market, error) {
			captured = filters
			return []*dmarkets.Market{}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v0/markets?created_by=alice&limit=15&offset=4", nil)
	rr := httptest.NewRecorder()

	handler.ListMarkets(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	expected := dmarkets.ListFilters{CreatedBy: "alice", Limit: 15, Offset: 4}
	if captured != expected {
		t.Fatalf("expected filters %+v, got %+v", expected, captured)
	}
}

func TestHandlerSearchMarketsInvalidRequestEnvelope(t *testing.T) {
	handler := NewHandler(&handlerServiceStub{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/v0/markets/search", nil)
	rr := httptest.NewRecorder()

	handler.SearchMarkets(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	var resp handlers.FailureEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode failure envelope: %v", err)
	}
	if resp.Reason != string(handlers.ReasonInvalidRequest) {
		t.Fatalf("expected %q, got %q", handlers.ReasonInvalidRequest, resp.Reason)
	}
}

func TestHandlerGetDetailsNotFoundEnvelope(t *testing.T) {
	handler := NewHandler(&handlerServiceStub{
		getMarketDetailsFn: func(context.Context, int64) (*dmarkets.MarketOverview, error) {
			return nil, dmarkets.ErrMarketNotFound
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v0/markets/42", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "42"})
	rr := httptest.NewRecorder()

	handler.GetDetails(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}

	var resp handlers.FailureEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode failure envelope: %v", err)
	}
	if resp.Reason != string(handlers.ReasonNotFound) {
		t.Fatalf("expected %q, got %q", handlers.ReasonNotFound, resp.Reason)
	}
}

func TestHandlerResolveMarketAuthorizationDeniedEnvelope(t *testing.T) {
	handler := NewHandler(&handlerServiceStub{
		resolveMarketFn: func(context.Context, int64, string, string) error {
			return dmarkets.ErrUnauthorized
		},
	}, authStub{
		currentUserFn: func(*http.Request) (*dusers.User, *authsvc.HTTPError) {
			return &dusers.User{Username: "other-user"}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/v0/markets/42/resolve", bytes.NewBufferString(`{"resolution":"YES"}`))
	req = mux.SetURLVars(req, map[string]string{"id": "42"})
	rr := httptest.NewRecorder()

	handler.ResolveMarket(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}

	var resp handlers.FailureEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode failure envelope: %v", err)
	}
	if resp.Reason != string(handlers.ReasonAuthorizationDenied) {
		t.Fatalf("expected %q, got %q", handlers.ReasonAuthorizationDenied, resp.Reason)
	}
}

func TestHandlerProjectProbabilityUsesQueryParameters(t *testing.T) {
	var captured dmarkets.ProbabilityProjectionRequest
	handler := NewHandler(&handlerServiceStub{
		projectProbabilityFn: func(ctx context.Context, req dmarkets.ProbabilityProjectionRequest) (*dmarkets.ProbabilityProjection, error) {
			captured = req
			return &dmarkets.ProbabilityProjection{
				CurrentProbability:   0.41,
				ProjectedProbability: 0.67,
			}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v0/markets/77/projection?amount=55&outcome=YES", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "77"})
	rr := httptest.NewRecorder()

	handler.ProjectProbability(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	expectedReq := dmarkets.ProbabilityProjectionRequest{MarketID: 77, Amount: 55, Outcome: "YES"}
	if captured != expectedReq {
		t.Fatalf("expected request %+v, got %+v", expectedReq, captured)
	}

	var resp dto.ProbabilityProjectionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.MarketID != 77 || resp.Amount != 55 || resp.Outcome != "YES" || resp.ProjectedProbability != 0.67 {
		t.Fatalf("unexpected response payload: %+v", resp)
	}
}
