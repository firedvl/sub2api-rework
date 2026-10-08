//go:build unit

package service

import (
	"context"
	"errors"
	"html"
	"io"
	"mime/quotedprintable"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type resetTokenCacheStub struct {
	emailCacheStub
	stored          *PasswordResetTokenData
	consumeErr      error
	consumeMismatch bool
	consumedHash    string
	setErr          error
}

func (s *resetTokenCacheStub) GetPasswordResetToken(context.Context, string) (*PasswordResetTokenData, error) {
	return s.stored, nil
}

func (s *resetTokenCacheStub) SetPasswordResetToken(_ context.Context, _ string, data *PasswordResetTokenData, _ time.Duration) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.stored = data
	return nil
}

func (s *resetTokenCacheStub) ConsumePasswordResetToken(_ context.Context, _ string, hash string) (bool, error) {
	s.consumedHash = hash
	if s.consumeErr != nil {
		return false, s.consumeErr
	}
	if s.consumeMismatch || s.stored == nil || s.stored.Token != hash {
		return false, nil
	}
	s.stored = nil
	return true, nil
}

func TestEmailService_ResetTokenHashAndConsume(t *testing.T) {
	ctx := context.Background()
	const token = "raw-token"
	for _, variant := range []string{"valid", "legacy", "mismatch", "failure", "empty"} {
		t.Run(variant, func(t *testing.T) {
			cache := &resetTokenCacheStub{stored: &PasswordResetTokenData{Token: hashPasswordResetToken(token)}}
			input := token
			switch variant {
			case "legacy":
				cache.stored.Token = token
			case "mismatch":
				cache.consumeMismatch = true
			case "failure":
				cache.consumeErr = errors.New("cache unavailable")
			case "empty":
				input = ""
			}
			svc := NewEmailService(nil, cache)
			err := svc.ConsumePasswordResetToken(ctx, "user@example.com", input)
			if variant == "valid" {
				require.NoError(t, err)
				require.Equal(t, hashPasswordResetToken(token), cache.consumedHash)
				require.Nil(t, cache.stored)
				require.ErrorIs(t, svc.ConsumePasswordResetToken(ctx, "user@example.com", token), ErrInvalidResetToken)
			} else {
				require.ErrorIs(t, err, ErrInvalidResetToken)
				require.NotNil(t, cache.stored)
			}
		})
	}
}

func TestAuthService_ResetPasswordRequiresAtomicConsume(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(strconv.FormatBool(fail), func(t *testing.T) {
			user := &User{ID: 1, Email: "user@example.com", PasswordHash: "original", Status: StatusActive}
			repo := &userRepoStub{user: user}
			cache := &resetTokenCacheStub{stored: &PasswordResetTokenData{Token: hashPasswordResetToken("token")}}
			if fail {
				cache.consumeErr = errors.New("cache unavailable")
			}
			svc := newAuthService(repo, map[string]string{
				SettingKeyEmailVerifyEnabled: "true", SettingKeyPasswordResetEnabled: "true",
			}, cache, nil)
			err := svc.ResetPassword(context.Background(), user.Email, "token", "new-password")
			if fail {
				require.ErrorIs(t, err, ErrInvalidResetToken)
				require.Empty(t, repo.updated)
				require.Equal(t, "original", user.PasswordHash)
				require.Zero(t, user.TokenVersion)
			} else {
				require.NoError(t, err)
				require.Len(t, repo.updated, 1)
				require.NotEqual(t, "original", user.PasswordHash)
				require.Equal(t, int64(1), user.TokenVersion)
				require.ErrorIs(t, svc.ResetPassword(context.Background(), user.Email, "token", "another-password"), ErrInvalidResetToken)
				require.Len(t, repo.updated, 1)
			}
		})
	}
}

func TestEmailService_ResetEmailPersistsDigestAndRotates(t *testing.T) {
	srv, port := startFakeSMTPServer(t, false, false)
	settings := &settingRepoStub{values: map[string]string{
		SettingKeySMTPHost: "127.0.0.1", SettingKeySMTPPort: strconv.Itoa(port),
		SettingKeySMTPFrom: "sender@example.com", SettingKeySMTPUsername: "sender@example.com",
		SettingKeySMTPPassword: "synthetic-password",
	}}
	cache := &resetTokenCacheStub{}
	svc := NewEmailService(settings, cache)
	ctx := context.Background()
	var firstToken string
	for i := 0; i < 2; i++ {
		require.NoError(t, svc.SendPasswordResetEmail(ctx, "user@example.com", "Test", "https://example.com/reset"))
		srv.mu.Lock()
		raw := strings.Join(srv.dataLines, "")
		srv.dataLines = nil
		srv.mu.Unlock()
		message, err := mail.ReadMessage(strings.NewReader(raw))
		require.NoError(t, err)
		decoded, err := io.ReadAll(quotedprintable.NewReader(message.Body))
		require.NoError(t, err)
		body := html.UnescapeString(string(decoded))
		match := regexp.MustCompile(`https://example.com/reset\?email=[^"<>\s]+`).FindString(body)
		parsed, err := url.Parse(match)
		require.NoError(t, err)
		token := parsed.Query().Get("token")
		require.Len(t, token, 64)
		require.Equal(t, hashPasswordResetToken(token), cache.stored.Token)
		require.NotEqual(t, token, cache.stored.Token)
		require.NoError(t, svc.VerifyPasswordResetToken(ctx, "user@example.com", token))
		if i == 0 {
			firstToken = token
		} else {
			require.NotEqual(t, firstToken, token)
			require.ErrorIs(t, svc.VerifyPasswordResetToken(ctx, "user@example.com", firstToken), ErrInvalidResetToken)
		}
	}
	cache.setErr = errors.New("cache unavailable")
	before := srv.conns.Load()
	require.Error(t, svc.SendPasswordResetEmail(ctx, "user@example.com", "Test", "https://example.com/reset"))
	require.Equal(t, before, srv.conns.Load(), "must not send an unpersisted link")
}

type notifyAttemptCacheStub struct {
	emailCacheStub
	attempts   int
	consumeErr error
	consumed   bool
}

func (s *notifyAttemptCacheStub) GetNotifyVerifyCode(context.Context, string) (*VerificationCodeData, error) {
	if s.consumed {
		return nil, nil
	}
	return &VerificationCodeData{Code: "123456", Attempts: s.attempts}, nil
}

func (s *notifyAttemptCacheStub) IncrNotifyVerifyCodeAttempts(_ context.Context, _ string, _ *VerificationCodeData, consume bool) (int, error) {
	if s.consumeErr != nil {
		return 0, s.consumeErr
	}
	s.attempts++
	s.consumed = consume && s.attempts <= MaxVerifyCodeAttempts
	return s.attempts, nil
}

func TestVerifyNotifyCode_AttemptCapAndSuccess(t *testing.T) {
	ctx := context.Background()
	cache := &notifyAttemptCacheStub{}
	for i := 1; i <= 5; i++ {
		err := verifyNotifyCode(ctx, cache, "user@example.com", "wrong")
		if i < 5 {
			require.ErrorIs(t, err, ErrInvalidVerifyCode)
		} else {
			require.ErrorIs(t, err, ErrVerifyCodeMaxAttempts)
		}
	}
	require.ErrorIs(t, verifyNotifyCode(ctx, cache, "user@example.com", "123456"), ErrVerifyCodeMaxAttempts)
	cache = &notifyAttemptCacheStub{attempts: 4}
	require.NoError(t, verifyNotifyCode(ctx, cache, "user@example.com", "123456"))
	require.True(t, cache.consumed)
	require.ErrorIs(t, verifyNotifyCode(ctx, cache, "user@example.com", "123456"), ErrInvalidVerifyCode)
	cache = &notifyAttemptCacheStub{consumeErr: errors.New("cache unavailable")}
	require.ErrorIs(t, verifyNotifyCode(ctx, cache, "user@example.com", "123456"), ErrInvalidVerifyCode)
	require.False(t, cache.consumed)
}
