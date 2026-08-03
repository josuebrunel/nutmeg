package api

import ezauthmodels "github.com/josuebrunel/ezauth/pkg/db/models"

type AccountResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func toAccountResponse(u *ezauthmodels.User) AccountResponse {
	return AccountResponse{ID: u.ID, Email: u.Email, Username: u.Username, FirstName: u.FirstName, LastName: u.LastName}
}

type AccountUpdateRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	Email     string `json:"email"`
}

type PasswordChangeRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}
