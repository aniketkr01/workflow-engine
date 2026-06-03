package handlers

import (
	"net/http"

	"github.com/aniketkr01/workflow-engine/internal/auth"
	"github.com/aniketkr01/workflow-engine/internal/domain"
	"github.com/aniketkr01/workflow-engine/internal/repository"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	userRepo   repository.UserRepository
	jwtManager *auth.JWTManager
}

func NewAuthHandler(userRepo repository.UserRepository, jwtManager *auth.JWTManager) *AuthHandler {
	return &AuthHandler{userRepo: userRepo, jwtManager: jwtManager}
}

type registerRequest struct {
	Email    string      `json:"email" binding:"required,email"`
	Password string      `json:"password" binding:"required,min=8"`
	Role     domain.Role `json:"role"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Register
// POST /auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	role := req.Role
	if role == "" {
		role = domain.RoleOperator
	}

	user := &domain.User{
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         role,
	}
	if err := h.userRepo.Create(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}

	token, err := h.jwtManager.Generate(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"user":  user,
	})
}

// Login godoc
// POST /auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.userRepo.GetByEmail(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := h.jwtManager.Generate(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  user,
	})
}

// Me godoc
// GET /auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	claims, ok := auth.GetClaims(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, err := claims.GetUserID()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id in token"})
		return
	}
	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}
