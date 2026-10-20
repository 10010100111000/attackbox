package control

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
)

type RegisterReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type LoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResp struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

func (s *ControlHTTPServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req RegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds maximum allowed size (1MB)")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	if len(req.Username) < 3 || len(req.Password) < 6 || !strings.Contains(req.Email, "@") {
		writeError(w, http.StatusBadRequest, "username must be >=3 chars, password >=6 chars, and email must be valid")
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user, err := s.db.CreateUser(req.Username, hash, req.Email, "")
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			writeError(w, http.StatusConflict, "username or email already registered")
			return
		}
		log.Printf("[AUTH] Failed to create user %s: %v", req.Username, err)
		writeError(w, http.StatusInternalServerError, "failed to create user account")
		return
	}

	claims := JWTClaims{
		Sub:          user.ID,
		Username:     user.Username,
		Role:         user.Role,
		TokenVersion: user.TokenVersion,
		Exp:          time.Now().Add(time.Duration(s.cfg.Auth.JWTExpireHours) * time.Hour).Unix(),
	}
	token, err := SignJWT(s.privKey, claims)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusCreated, AuthResp{
		Token: token,
		User:  user,
	})
}

func (s *ControlHTTPServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req LoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds maximum allowed size (1MB)")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.db.GetUserByUsername(strings.TrimSpace(req.Username))
	hashToCompare := dummyBcryptHash
	if err == nil && user != nil {
		hashToCompare = user.PasswordHash
	}

	// Constant-time bcrypt computation to eliminate username enumeration timing side channels
	passwordMatch := CheckPassword(hashToCompare, req.Password)

	if err != nil || user == nil || !passwordMatch {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	if user.Status != 1 {
		writeError(w, http.StatusForbidden, "account suspended")
		return
	}

	claims := JWTClaims{
		Sub:          user.ID,
		Username:     user.Username,
		Role:         user.Role,
		TokenVersion: user.TokenVersion,
		Exp:          time.Now().Add(time.Duration(s.cfg.Auth.JWTExpireHours) * time.Hour).Unix(),
	}
	token, err := SignJWT(s.privKey, claims)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, AuthResp{
		Token: token,
		User:  user,
	})
}

func (s *ControlHTTPServer) handleMe(w http.ResponseWriter, r *http.Request) {
	uid, _, _, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := s.db.GetUserByID(uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, user)
}
