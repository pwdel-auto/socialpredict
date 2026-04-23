package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"socialpredict/handlers"
	statshandlers "socialpredict/handlers/stats"
	analytics "socialpredict/internal/domain/analytics"
	configsvc "socialpredict/internal/service/config"
	"socialpredict/models"
	"socialpredict/models/modelstesting"
	"socialpredict/security"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

const testOpenAPISpec = "openapi: 3.0.3\ninfo:\n  title: test\n  version: 0.0.0\n"

var testSwaggerUIFS = fstest.MapFS{
	"swagger-ui/index.html": &fstest.MapFile{Data: []byte("<!doctype html><html><body>swagger</body></html>")},
}

func buildTestHandler(t *testing.T, db *gorm.DB) http.Handler {
	t.Helper()

	econConfig := modelstesting.GenerateEconomicConfig()
	return buildTestHandlerWithConfig(t, db, econConfig)
}

func loadServerOpenAPIDoc(t *testing.T) *openapi3.T {
	t.Helper()

	specPath := filepath.Join("..", "docs", "openapi.yaml")
	loader := &openapi3.Loader{IsExternalRefsAllowed: true}

	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("load OpenAPI document %s: %v", specPath, err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI document %s: %v", specPath, err)
	}

	return doc
}

func collectDocumentedOperations(doc *openapi3.T) map[string]struct{} {
	operations := make(map[string]struct{})
	for path, pathItem := range doc.Paths.Map() {
		for method := range pathItem.Operations() {
			operations[strings.ToUpper(method)+" "+path] = struct{}{}
		}
	}
	return operations
}

func collectRegisteredOperations(t *testing.T, router *mux.Router) map[string]struct{} {
	t.Helper()

	operations := make(map[string]struct{})
	if err := router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		pathTemplate, err := route.GetPathTemplate()
		if err != nil {
			return err
		}

		methods, err := route.GetMethods()
		if err != nil {
			return err
		}

		for _, method := range methods {
			operations[method+" "+pathTemplate] = struct{}{}
		}
		return nil
	}); err != nil {
		t.Fatalf("walk registered routes: %v", err)
	}

	return operations
}

func sortedOperationDiff(left, right map[string]struct{}) []string {
	var diff []string
	for operation := range left {
		if _, ok := right[operation]; !ok {
			diff = append(diff, operation)
		}
	}
	sort.Strings(diff)
	return diff
}

func seedServerTestData(t *testing.T, db *gorm.DB) {
	t.Helper()

	creator := modelstesting.GenerateUser("creator", 1000)
	if err := db.Create(&creator).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	market := modelstesting.GenerateMarket(1, creator.Username)
	if err := db.Create(&market).Error; err != nil {
		t.Fatalf("seed market: %v", err)
	}
}

func TestServerRegistersAndServesCoreRoutes(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-secret-key")

	db := modelstesting.NewFakeDB(t)
	seedServerTestData(t, db)

	handler := buildTestHandler(t, db)

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{"health", "/health", http.StatusOK},
		{"openapi", "/openapi.yaml", http.StatusOK},
		{"swagger redirect", "/swagger", http.StatusMovedPermanently},
		{"swagger ui", "/swagger/", http.StatusOK},
		{"home", "/v0/home", http.StatusOK},
		{"setup frontend", "/v0/setup/frontend", http.StatusOK},
		{"markets", "/v0/markets?status=ACTIVE", http.StatusOK},
		{"markets active alias", "/v0/markets/active", http.StatusOK},
		{"userinfo", "/v0/userinfo/creator", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d (body: %s)", tt.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestServerReportingAndContentRoutesRemainPublic(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-secret-key")

	db := modelstesting.NewFakeDB(t)
	seedServerTestData(t, db)

	item := models.HomepageContent{
		Slug:    "home",
		Title:   "Welcome",
		Format:  "html",
		HTML:    "<p>Hello</p>",
		Version: 1,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("seed homepage content: %v", err)
	}

	handler := buildTestHandler(t, db)

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "home", path: "/v0/home", wantStatus: http.StatusOK},
		{name: "stats", path: "/v0/stats", wantStatus: http.StatusOK},
		{name: "system metrics", path: "/v0/system/metrics", wantStatus: http.StatusOK},
		{name: "global leaderboard", path: "/v0/global/leaderboard", wantStatus: http.StatusOK},
		{name: "content home", path: "/v0/content/home", wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d (body: %s)", tt.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestOpenAPISpecMatchesRegisteredRoutes(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-secret-key")

	db := modelstesting.NewFakeDB(t)
	seedServerTestData(t, db)

	configService := configsvc.NewStaticService(modelstesting.GenerateEconomicConfig())
	router, err := buildRouter([]byte(testOpenAPISpec), testSwaggerUIFS, db, configService, security.NewSecurityService())
	if err != nil {
		t.Fatalf("build router: %v", err)
	}

	documented := collectDocumentedOperations(loadServerOpenAPIDoc(t))
	registered := collectRegisteredOperations(t, router)

	missingFromSpec := sortedOperationDiff(registered, documented)
	staleInSpec := sortedOperationDiff(documented, registered)

	if len(missingFromSpec) == 0 && len(staleInSpec) == 0 {
		return
	}

	var message strings.Builder
	if len(missingFromSpec) > 0 {
		message.WriteString("registered routes missing from OpenAPI:\n")
		message.WriteString(strings.Join(missingFromSpec, "\n"))
	}
	if len(staleInSpec) > 0 {
		if message.Len() > 0 {
			message.WriteString("\n")
		}
		message.WriteString("OpenAPI operations without a registered route:\n")
		message.WriteString(strings.Join(staleInSpec, "\n"))
	}

	t.Fatalf("%s", message.String())
}

func TestServerConfigDerivedRoutesUseInjectedConfigService(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-secret-key")

	db := modelstesting.NewFakeDB(t)
	regularUser := modelstesting.GenerateUser("regular", 0)
	regularUser.UserType = "REGULAR"
	if err := db.Create(&regularUser).Error; err != nil {
		t.Fatalf("seed regular user: %v", err)
	}

	config := modelstesting.GenerateEconomicConfig()
	config.Economics.MarketIncentives.CreateMarketCost = 77
	config.Economics.User.InitialAccountBalance = 250
	config.Economics.User.MaximumDebtAllowed = 900
	config.Frontend.Charts.SigFigs = 1

	handler := buildTestHandlerWithConfig(t, db, config)

	setupReq := httptest.NewRequest(http.MethodGet, "/v0/setup", nil)
	setupRec := httptest.NewRecorder()
	handler.ServeHTTP(setupRec, setupReq)
	if setupRec.Code != http.StatusOK {
		t.Fatalf("expected setup status 200, got %d with body %s", setupRec.Code, setupRec.Body.String())
	}

	var economics struct {
		MarketIncentives struct {
			CreateMarketCost int64 `json:"CreateMarketCost"`
		} `json:"MarketIncentives"`
	}
	if err := json.Unmarshal(setupRec.Body.Bytes(), &economics); err != nil {
		t.Fatalf("decode /v0/setup response: %v", err)
	}
	if economics.MarketIncentives.CreateMarketCost != 77 {
		t.Fatalf("expected injected setup createMarketCost 77, got %d", economics.MarketIncentives.CreateMarketCost)
	}

	frontendReq := httptest.NewRequest(http.MethodGet, "/v0/setup/frontend", nil)
	frontendRec := httptest.NewRecorder()
	handler.ServeHTTP(frontendRec, frontendReq)
	if frontendRec.Code != http.StatusOK {
		t.Fatalf("expected frontend setup status 200, got %d with body %s", frontendRec.Code, frontendRec.Body.String())
	}

	var frontend struct {
		Charts struct {
			SigFigs int `json:"sigFigs"`
		} `json:"charts"`
	}
	if err := json.Unmarshal(frontendRec.Body.Bytes(), &frontend); err != nil {
		t.Fatalf("decode /v0/setup/frontend response: %v", err)
	}
	if frontend.Charts.SigFigs != 2 {
		t.Fatalf("expected clamped frontend sig figs 2, got %d", frontend.Charts.SigFigs)
	}

	statsReq := httptest.NewRequest(http.MethodGet, "/v0/stats", nil)
	statsRec := httptest.NewRecorder()
	handler.ServeHTTP(statsRec, statsReq)
	if statsRec.Code != http.StatusOK {
		t.Fatalf("expected stats status 200, got %d with body %s", statsRec.Code, statsRec.Body.String())
	}

	var stats handlers.SuccessEnvelope[statshandlers.StatsResponse]
	if err := json.Unmarshal(statsRec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode /v0/stats response: %v", err)
	}
	if stats.Result.FinancialStats.TotalMoney != 250 {
		t.Fatalf("expected injected stats totalMoney 250, got %d", stats.Result.FinancialStats.TotalMoney)
	}
	if stats.Result.SetupConfiguration.MaximumDebtAllowed != 900 {
		t.Fatalf("expected injected stats maximumDebtAllowed 900, got %d", stats.Result.SetupConfiguration.MaximumDebtAllowed)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/v0/system/metrics", nil)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("expected metrics status 200, got %d with body %s", metricsRec.Code, metricsRec.Body.String())
	}

	var metrics handlers.SuccessEnvelope[analytics.SystemMetrics]
	if err := json.Unmarshal(metricsRec.Body.Bytes(), &metrics); err != nil {
		t.Fatalf("decode /v0/system/metrics response: %v", err)
	}
	if metrics.Result.MoneyCreated.UserDebtCapacity.Value != 900 {
		t.Fatalf("expected injected metrics debt capacity 900, got %d", metrics.Result.MoneyCreated.UserDebtCapacity.Value)
	}
}

func TestServerBlocksProtectedProfileRoutesWhenPasswordChangeRequired(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-secret-key-for-testing")

	db := modelstesting.NewFakeDB(t)
	seedServerTestData(t, db)

	user := modelstesting.GenerateUser("mustchange", 1000)
	user.MustChangePassword = true
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	handler := buildTestHandler(t, db)
	token := modelstesting.GenerateValidJWT(user.Username)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "private profile", method: http.MethodGet, path: "/v0/privateprofile"},
		{name: "change display name", method: http.MethodPost, path: "/v0/profilechange/displayname", body: `{"displayName":"New Name"}`},
		{name: "change emoji", method: http.MethodPost, path: "/v0/profilechange/emoji", body: `{"emoji":":)"}`},
		{name: "change description", method: http.MethodPost, path: "/v0/profilechange/description", body: `{"description":"updated description"}`},
		{name: "change links", method: http.MethodPost, path: "/v0/profilechange/links", body: `{"personalLink1":"https://example.com"}`},
		{name: "user position", method: http.MethodGet, path: "/v0/userposition/1"},
		{name: "place bet", method: http.MethodPost, path: "/v0/bet", body: `{"marketId":1,"amount":5,"outcome":"YES"}`},
		{name: "sell shares", method: http.MethodPost, path: "/v0/sell", body: `{"marketId":1,"amount":1,"outcome":"YES"}`},
		{name: "admin create user", method: http.MethodPost, path: "/v0/admin/createuser", body: `{"username":"newuser123"}`},
		{name: "admin update home", method: http.MethodPut, path: "/v0/admin/content/home", body: `{"title":"Home","format":"markdown","markdown":"# Home","version":1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			req.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusForbidden {
				t.Fatalf("expected status 403, got %d (body: %s)", rr.Code, rr.Body.String())
			}

			var response handlers.FailureEnvelope
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode failure envelope: %v", err)
			}
			if response.OK || response.Reason != string(handlers.ReasonPasswordChangeRequired) {
				t.Fatalf("expected password-change failure envelope, got %+v", response)
			}
		})
	}
}

func TestServerBlocksProtectedMarketActionsWhenPasswordChangeRequired(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-secret-key-for-testing")

	db := modelstesting.NewFakeDB(t)
	seedServerTestData(t, db)

	user := modelstesting.GenerateUser("mustchangemarket", 1000)
	user.MustChangePassword = true
	if err := user.HashPassword("OldPassword123"); err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	handler := buildTestHandler(t, db)
	token := modelstesting.GenerateValidJWT(user.Username)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "create market",
			method: http.MethodPost,
			path:   "/v0/markets",
			body:   `{"questionTitle":"Will it rain?","description":"weather","outcomeType":"BINARY","resolutionDateTime":"2099-01-01T00:00:00Z","yesLabel":"Yes","noLabel":"No"}`,
		},
		{
			name:   "resolve market",
			method: http.MethodPost,
			path:   "/v0/markets/1/resolve",
			body:   `{"resolution":"YES"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			req.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusForbidden {
				t.Fatalf("expected status 403, got %d (body: %s)", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestServerAllowsChangePasswordWhenPasswordChangeRequired(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-secret-key-for-testing")

	db := modelstesting.NewFakeDB(t)

	user := modelstesting.GenerateUser("needsreset", 1000)
	user.MustChangePassword = true
	if err := user.HashPassword("OldPassword123"); err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	handler := buildTestHandler(t, db)
	req := httptest.NewRequest(http.MethodPost, "/v0/changepassword", bytes.NewBufferString(`{"currentPassword":"OldPassword123","newPassword":"NewPassword123"}`))
	req.Header.Set("Authorization", "Bearer "+modelstesting.GenerateValidJWT(user.Username))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var response struct {
		OK     bool `json:"ok"`
		Result struct {
			Message string `json:"message"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode success envelope: %v", err)
	}
	if !response.OK || response.Result.Message != "Password changed successfully" {
		t.Fatalf("unexpected change-password response: %+v", response)
	}
}

func buildTestHandlerWithConfig(t *testing.T, db *gorm.DB, econConfig *configsvc.AppConfig) http.Handler {
	t.Helper()

	configService := configsvc.NewStaticService(econConfig)
	handler, err := buildHandler([]byte(testOpenAPISpec), testSwaggerUIFS, db, configService)
	if err != nil {
		t.Fatalf("build test handler: %v", err)
	}
	return handler
}
