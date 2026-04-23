package adminhandlers

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"socialpredict/handlers"
	"socialpredict/handlers/failurereason"
	dusers "socialpredict/internal/domain/users"
	authsvc "socialpredict/internal/service/auth"
	configsvc "socialpredict/internal/service/config"
	"socialpredict/models"
	"socialpredict/security"

	"github.com/brianvoe/gofakeit"
	"gorm.io/gorm"
)

func AddUserHandler(db *gorm.DB, configService configsvc.Service, auth authsvc.Authenticator) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			_ = handlers.WriteFailure(w, http.StatusMethodNotAllowed, handlers.ReasonMethodNotAllowed)
			return
		}

		responseData, handlerErr := processAddUser(r, db, configService, auth)
		if handlerErr != nil {
			_ = handlers.WriteFailure(w, handlerErr.statusCode, handlerErr.reason)
			if handlerErr.logErr != nil {
				log.Printf("AddUserHandler: %v", handlerErr.logErr)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responseData)
	}
}

type handlerError struct {
	reason     handlers.FailureReason
	statusCode int
	logErr     error
}

func processAddUser(r *http.Request, db *gorm.DB, configService configsvc.Service, auth authsvc.Authenticator) (map[string]interface{}, *handlerError) {
	securityService := security.NewSecurityService()
	req, decodeErr := decodeAddUserRequest(r)
	if decodeErr != nil {
		return nil, &handlerError{reason: handlers.ReasonInvalidRequest, statusCode: http.StatusBadRequest, logErr: decodeErr}
	}

	if err := validateAddUserUsername(securityService, req.Username); err != nil {
		return nil, &handlerError{reason: handlers.ReasonValidationFailed, statusCode: http.StatusBadRequest, logErr: err}
	}
	req.Username, _ = securityService.Sanitizer.SanitizeUsername(req.Username)

	if db == nil {
		return nil, &handlerError{reason: handlers.ReasonInternalError, statusCode: http.StatusInternalServerError}
	}

	if auth == nil {
		return nil, &handlerError{reason: handlers.ReasonInternalError, statusCode: http.StatusInternalServerError}
	}
	if _, httpErr := auth.RequireAdmin(r); httpErr != nil {
		return nil, &handlerError{reason: failurereason.FromAuthHTTPError(httpErr), statusCode: httpErr.StatusCode, logErr: httpErr}
	}

	if configService == nil {
		return nil, &handlerError{reason: handlers.ReasonInternalError, statusCode: http.StatusInternalServerError}
	}

	appConfig := configService.Current()
	user := buildNewUser(db, req.Username, appConfig)

	if err := checkUniqueFields(db, &user); err != nil {
		return nil, &handlerError{reason: handlers.ReasonValidationFailed, statusCode: http.StatusBadRequest, logErr: err}
	}

	password, err := generateAndHashPassword(&user)
	if err != nil {
		return nil, &handlerError{reason: handlers.ReasonInternalError, statusCode: http.StatusInternalServerError, logErr: err}
	}

	if result := db.Create(&user); result.Error != nil {
		return nil, &handlerError{reason: handlers.ReasonInternalError, statusCode: http.StatusInternalServerError, logErr: result.Error}
	}

	responseData := map[string]interface{}{
		"message":  "User created successfully",
		"username": user.Username,
		"password": password,
		"usertype": user.UserType,
	}
	return responseData, nil
}

type addUserRequest struct {
	Username string `json:"username" validate:"required,min=3,max=30,username"`
}

func decodeAddUserRequest(r *http.Request) (addUserRequest, error) {
	var req addUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return addUserRequest{}, fmt.Errorf("Error decoding request body")
	}
	return req, nil
}

func validateAddUserUsername(securityService *security.SecurityService, username string) error {
	if err := securityService.Validator.ValidateStruct(addUserRequest{Username: username}); err != nil {
		return err
	}
	_, err := securityService.Sanitizer.SanitizeUsername(username)
	return err
}

func buildNewUser(db *gorm.DB, username string, appConfig *configsvc.AppConfig) models.User {
	return models.User{
		PublicUser: models.PublicUser{
			Username:              username,
			DisplayName:           dusers.UniqueDisplayName(db),
			UserType:              "REGULAR",
			InitialAccountBalance: appConfig.Economics.User.InitialAccountBalance,
			AccountBalance:        appConfig.Economics.User.InitialAccountBalance,
			PersonalEmoji:         randomEmoji(),
		},
		PrivateUser: models.PrivateUser{
			Email:  dusers.UniqueEmail(db),
			APIKey: dusers.GenerateUniqueAPIKey(db),
		},
		MustChangePassword: true,
	}
}

func generateAndHashPassword(user *models.User) (string, error) {
	password := gofakeit.Password(true, true, true, false, false, 12)
	if err := user.HashPassword(password); err != nil {
		return "", fmt.Errorf("Failed to hash password")
	}
	return password, nil
}

func checkUniqueFields(db *gorm.DB, user *models.User) error {
	// Check for existing users with the same username, display name, email, or API key.
	var count int64
	db.Model(&models.User{}).Where(
		"username = ? OR display_name = ? OR email = ? OR api_key = ?",
		user.Username, user.DisplayName, user.Email, user.APIKey,
	).Count(&count)

	if count > 0 {
		return fmt.Errorf("username, display name, email, or API key already in use")
	}

	return nil
}

func randomEmoji() string {
	emojis := []string{"😀", "😃", "😄", "😁", "😆"}
	return emojis[rand.Intn(len(emojis))]
}
