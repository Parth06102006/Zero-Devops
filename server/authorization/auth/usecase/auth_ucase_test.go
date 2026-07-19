package usecase

import (
	"Zero_Devops/server/domain"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

type mockUserRepository struct {
	users      map[int64]domain.User
	providerID map[int64]domain.User
}

func newMockUserRepository() *mockUserRepository {
	return &mockUserRepository{
		users:      make(map[int64]domain.User),
		providerID: make(map[int64]domain.User),
	}
}

func (m *mockUserRepository) GetByID(ctx context.Context, id int64) (domain.User, error) {
	if user, ok := m.users[id]; ok {
		return user, nil
	}
	return domain.User{}, domain.ErrNotFound
}

func (m *mockUserRepository) GetByUsername(ctx context.Context, username string) (domain.User, error) {
	for _, user := range m.users {
		if user.Username == username {
			return user, nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}

func (m *mockUserRepository) GetProviderByID(ctx context.Context, providerID int64) (domain.User, error) {
	if user, ok := m.providerID[providerID]; ok {
		return user, nil
	}
	return domain.User{}, domain.ErrNotFound
}

func (m *mockUserRepository) Store(ctx context.Context, u *domain.User) error {
	u.ID = int64(len(m.users) + 1)
	m.users[u.ID] = *u
	m.providerID[u.ProviderID] = *u
	return nil
}

func (m *mockUserRepository) UpdateRefreshToken(ctx context.Context, id int64, refreshToken string) error {
	if user, ok := m.users[id]; ok {
		user.RefreshToken = refreshToken
		m.users[id] = user
		m.providerID[user.ProviderID] = user
		return nil
	}
	return domain.ErrNotFound
}

type mockOAuthProvider struct {
	token string
	user  *domain.OAuthUser
	err   error
}

func (m *mockOAuthProvider) ExchangeCode(ctx context.Context, code string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.token, nil
}

func (m *mockOAuthProvider) GetUser(ctx context.Context, accessToken string) (*domain.OAuthUser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.user, nil
}

func setJWTSecret() {
	viper.Set("JWT_SECRET", "test-secret-key-for-testing")
}

func generateTestToken(userID int64, exp time.Time) string {
	secretKey := []byte(viper.GetString("JWT_SECRET"))
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   "test@example.com",
		"exp":     exp.Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, _ := token.SignedString(secretKey)
	return signedToken
}

func TestHandleOAuthCallback_NewUser(t *testing.T) {
	setJWTSecret()

	mockRepo := newMockUserRepository()
	providers := map[string]domain.OAuthProvider{
		"github": &mockOAuthProvider{
			token: "provider-token",
			user: &domain.OAuthUser{
				Provider:  "github",
				ProviderID: 12345,
				Username:  "testuser",
				Email:     "test@example.com",
				AvatarURL: "https://example.com/avatar.png",
			},
		},
	}

	uc := NewAuthUsecase(mockRepo, providers, time.Second*5)
	ctx := context.Background()

	resp, err := uc.HandleOAuthCallback(ctx, "test-code", "github")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.AccessToken == "" {
		t.Error("expected access token to be set")
	}
	if resp.RefreshToken == "" {
		t.Error("expected refresh token to be set")
	}
}

func TestHandleOAuthCallback_UnsupportedProvider(t *testing.T) {
	setJWTSecret()

	mockRepo := newMockUserRepository()
	providers := map[string]domain.OAuthProvider{}

	uc := NewAuthUsecase(mockRepo, providers, time.Second*5)
	ctx := context.Background()

	_, err := uc.HandleOAuthCallback(ctx, "test-code", "unsupported")
	if !errors.Is(err, domain.ErrProviderNotSupported) {
		t.Fatalf("expected ErrProviderNotSupported, got %v", err)
	}
}

func TestHandleOAuthCallback_ExistingUser(t *testing.T) {
	setJWTSecret()

	mockRepo := newMockUserRepository()
	existingUser := domain.User{
		ID:          1,
		ProviderID:  12345,
		Provider:    "github",
		Username:    "testuser",
		Email:       "test@example.com",
		AvatarURL:   "https://example.com/avatar.png",
		CreatedAt:   time.Now(),
		RefreshToken: "old-refresh-token",
	}
	mockRepo.users[1] = existingUser
	mockRepo.providerID[12345] = existingUser

	providers := map[string]domain.OAuthProvider{
		"github": &mockOAuthProvider{
			token: "provider-token",
			user: &domain.OAuthUser{
				Provider:  "github",
				ProviderID: 12345,
				Username:  "testuser",
				Email:     "test@example.com",
				AvatarURL: "https://example.com/avatar.png",
			},
		},
	}

	uc := NewAuthUsecase(mockRepo, providers, time.Second*5)
	ctx := context.Background()

	resp, err := uc.HandleOAuthCallback(ctx, "test-code", "github")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.AccessToken == "" {
		t.Error("expected access token to be set")
	}
}

func TestRefreshToken_Success(t *testing.T) {
	setJWTSecret()

	mockRepo := newMockUserRepository()
	user := domain.User{
		ID:           1,
		ProviderID:   12345,
		Provider:     "github",
		Username:     "testuser",
		Email:        "test@example.com",
		CreatedAt:    time.Now(),
		RefreshToken: "valid-refresh-token",
	}
	mockRepo.users[1] = user
	mockRepo.providerID[12345] = user

	providers := map[string]domain.OAuthProvider{}

	uc := NewAuthUsecase(mockRepo, providers, time.Second*5)
	ctx := context.Background()

	refreshToken := generateTestToken(1, time.Now().Add(7*24*time.Hour))
	user.RefreshToken = refreshToken
	mockRepo.users[1] = user

	resp, err := uc.RefreshToken(ctx, refreshToken)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.AccessToken == "" {
		t.Error("expected access token to be set")
	}
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	setJWTSecret()

	mockRepo := newMockUserRepository()
	providers := map[string]domain.OAuthProvider{}

	uc := NewAuthUsecase(mockRepo, providers, time.Second*5)
	ctx := context.Background()

	_, err := uc.RefreshToken(ctx, "invalid-token")
	if !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestRefreshToken_UserNotFound(t *testing.T) {
	setJWTSecret()

	mockRepo := newMockUserRepository()
	providers := map[string]domain.OAuthProvider{}

	uc := NewAuthUsecase(mockRepo, providers, time.Second*5)
	ctx := context.Background()

	refreshToken := generateTestToken(999, time.Now().Add(7*24*time.Hour))

	_, err := uc.RefreshToken(ctx, refreshToken)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestLogout_Success(t *testing.T) {
	setJWTSecret()

	mockRepo := newMockUserRepository()
	user := domain.User{
		ID:           1,
		ProviderID:   12345,
		Provider:     "github",
		Username:     "testuser",
		Email:        "test@example.com",
		CreatedAt:    time.Now(),
		RefreshToken: "valid-refresh-token",
	}
	mockRepo.users[1] = user
	mockRepo.providerID[12345] = user

	providers := map[string]domain.OAuthProvider{}

	uc := NewAuthUsecase(mockRepo, providers, time.Second*5)
	ctx := context.Background()

	accessToken := generateTestToken(1, time.Now().Add(15*time.Minute))

	err := uc.Logout(ctx, accessToken)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	updatedUser, _ := mockRepo.GetByID(ctx, 1)
	if updatedUser.RefreshToken != "" {
		t.Error("expected refresh token to be cleared")
	}
}

func TestLogout_InvalidToken(t *testing.T) {
	setJWTSecret()

	mockRepo := newMockUserRepository()
	providers := map[string]domain.OAuthProvider{}

	uc := NewAuthUsecase(mockRepo, providers, time.Second*5)
	ctx := context.Background()

	err := uc.Logout(ctx, "invalid-token")
	if !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestGetCurrentUser_Success(t *testing.T) {
	setJWTSecret()

	mockRepo := newMockUserRepository()
	user := domain.User{
		ID:          1,
		ProviderID:  12345,
		Provider:    "github",
		Username:    "testuser",
		Email:       "test@example.com",
		AvatarURL:   "https://example.com/avatar.png",
		CreatedAt:   time.Now(),
	}
	mockRepo.users[1] = user
	mockRepo.providerID[12345] = user

	providers := map[string]domain.OAuthProvider{}

	uc := NewAuthUsecase(mockRepo, providers, time.Second*5)
	ctx := context.Background()

	accessToken := generateTestToken(1, time.Now().Add(15*time.Minute))

	resp, err := uc.GetCurrentUser(ctx, accessToken)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.ID != 1 {
		t.Errorf("expected user ID 1, got %d", resp.ID)
	}
	if resp.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", resp.Username)
	}
	if resp.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", resp.Email)
	}
}

func TestGetCurrentUser_InvalidToken(t *testing.T) {
	setJWTSecret()

	mockRepo := newMockUserRepository()
	providers := map[string]domain.OAuthProvider{}

	uc := NewAuthUsecase(mockRepo, providers, time.Second*5)
	ctx := context.Background()

	_, err := uc.GetCurrentUser(ctx, "invalid-token")
	if !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestGetCurrentUser_UserNotFound(t *testing.T) {
	setJWTSecret()

	mockRepo := newMockUserRepository()
	providers := map[string]domain.OAuthProvider{}

	uc := NewAuthUsecase(mockRepo, providers, time.Second*5)
	ctx := context.Background()

	accessToken := generateTestToken(999, time.Now().Add(15*time.Minute))

	_, err := uc.GetCurrentUser(ctx, accessToken)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}