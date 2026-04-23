package failurereason

import (
	"errors"
	"net/http"
	"testing"

	"socialpredict/handlers"
	dusers "socialpredict/internal/domain/users"
	authsvc "socialpredict/internal/service/auth"
)

func TestFromAuthHTTPError(t *testing.T) {
	tests := []struct {
		name string
		err  *authsvc.HTTPError
		want handlers.FailureReason
	}{
		{
			name: "nil",
			want: handlers.ReasonInternalError,
		},
		{
			name: "authentication required",
			err:  &authsvc.HTTPError{StatusCode: http.StatusUnauthorized, Message: "Invalid token"},
			want: handlers.ReasonAuthenticationRequired,
		},
		{
			name: "password change required",
			err:  &authsvc.HTTPError{StatusCode: http.StatusForbidden, Message: "Password change required"},
			want: handlers.ReasonPasswordChangeRequired,
		},
		{
			name: "authorization denied",
			err:  &authsvc.HTTPError{StatusCode: http.StatusForbidden, Message: "admin privileges required"},
			want: handlers.ReasonAuthorizationDenied,
		},
		{
			name: "not found",
			err:  &authsvc.HTTPError{StatusCode: http.StatusNotFound, Message: "User not found"},
			want: handlers.ReasonNotFound,
		},
		{
			name: "internal",
			err:  &authsvc.HTTPError{StatusCode: http.StatusInternalServerError, Message: "Failed to load user"},
			want: handlers.ReasonInternalError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromAuthHTTPError(tt.err); got != tt.want {
				t.Fatalf("expected reason %q, got %q", tt.want, got)
			}
		})
	}
}

func TestLooksLikeValidation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", want: false},
		{name: "invalid", err: errString("invalid outcome"), want: true},
		{name: "requirements", err: errString("password does not meet security requirements"), want: true},
		{name: "unsupported", err: errString("unsupported format"), want: true},
		{name: "internal", err: errString("database connection refused"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LooksLikeValidation(tt.err); got != tt.want {
				t.Fatalf("expected %t, got %t", tt.want, got)
			}
		})
	}
}

func TestFromUserError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantReason handlers.FailureReason
	}{
		{
			name:       "nil",
			wantStatus: http.StatusInternalServerError,
			wantReason: handlers.ReasonInternalError,
		},
		{
			name:       "not found",
			err:        dusers.ErrUserNotFound,
			wantStatus: http.StatusNotFound,
			wantReason: handlers.ReasonNotFound,
		},
		{
			name:       "invalid data",
			err:        dusers.ErrInvalidUserData,
			wantStatus: http.StatusBadRequest,
			wantReason: handlers.ReasonValidationFailed,
		},
		{
			name:       "invalid credentials",
			err:        dusers.ErrInvalidCredentials,
			wantStatus: http.StatusUnauthorized,
			wantReason: handlers.ReasonInvalidCredentials,
		},
		{
			name:       "unauthorized",
			err:        dusers.ErrUnauthorized,
			wantStatus: http.StatusForbidden,
			wantReason: handlers.ReasonAuthorizationDenied,
		},
		{
			name:       "validation-like string",
			err:        errors.New("new password must differ from the current password"),
			wantStatus: http.StatusBadRequest,
			wantReason: handlers.ReasonValidationFailed,
		},
		{
			name:       "internal fallback",
			err:        errors.New("database connection refused"),
			wantStatus: http.StatusInternalServerError,
			wantReason: handlers.ReasonInternalError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotReason := FromUserError(tt.err)
			if gotStatus != tt.wantStatus || gotReason != tt.wantReason {
				t.Fatalf("expected (%d, %q), got (%d, %q)", tt.wantStatus, tt.wantReason, gotStatus, gotReason)
			}
		})
	}
}

type errString string

func (e errString) Error() string {
	return string(e)
}
