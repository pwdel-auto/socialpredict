package usershandlers

import (
	"net/http"

	"socialpredict/handlers"
	"socialpredict/handlers/failurereason"
	"socialpredict/handlers/users/dto"
	dusers "socialpredict/internal/domain/users"
	authsvc "socialpredict/internal/service/auth"
)

func writeProfileError(w http.ResponseWriter, err error, field string) {
	statusCode, reason := failurereason.FromUserError(err)
	_ = handlers.WriteFailure(w, statusCode, reason)
}

func toPrivateUserResponse(user *dusers.User) dto.PrivateUserResponse {
	if user == nil {
		return dto.PrivateUserResponse{}
	}

	return dto.PrivateUserResponse{
		ID:                    user.ID,
		Username:              user.Username,
		DisplayName:           user.DisplayName,
		UserType:              user.UserType,
		InitialAccountBalance: user.InitialAccountBalance,
		AccountBalance:        user.AccountBalance,
		PersonalEmoji:         user.PersonalEmoji,
		Description:           user.Description,
		PersonalLink1:         user.PersonalLink1,
		PersonalLink2:         user.PersonalLink2,
		PersonalLink3:         user.PersonalLink3,
		PersonalLink4:         user.PersonalLink4,
		Email:                 user.Email,
		APIKey:                user.APIKey,
		MustChangePassword:    user.MustChangePassword,
	}
}

func profileAuthFailureReason(err *authsvc.HTTPError) handlers.FailureReason {
	return failurereason.FromAuthHTTPError(err)
}
