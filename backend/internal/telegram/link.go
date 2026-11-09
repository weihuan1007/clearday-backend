package telegram

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const linkLifetime = 10 * time.Minute

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{5,32}$`)

// PairHandler issues an expiring, single-use link only to the configured
// Cognito owner. The browser never receives the Telegram bot token.
func (bot *Bot) PairHandler(verify func(context.Context, string) (string, error)) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "private, no-store")
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", "POST")
			http.Error(response, "Method not allowed.", http.StatusMethodNotAllowed)
			return
		}
		token, ok := strings.CutPrefix(request.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			http.Error(response, "Sign in is required.", http.StatusUnauthorized)
			return
		}
		owner, err := verify(request.Context(), token)
		if err != nil {
			http.Error(response, "Sign in is required.", http.StatusUnauthorized)
			return
		}
		if owner != bot.owner {
			http.Error(response, "Telegram is unavailable for this account.", http.StatusForbidden)
			return
		}

		ctx, cancel := context.WithTimeout(request.Context(), 8*time.Second)
		defer cancel()
		user, err := bot.client.getMe(ctx)
		if err != nil || !user.IsBot || !usernamePattern.MatchString(user.Username) {
			http.Error(response, "Telegram is temporarily unavailable.", http.StatusServiceUnavailable)
			return
		}
		code := bot.newLink()
		response.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(response).Encode(map[string]string{"url": "https://t.me/" + user.Username + "?start=" + code})
	})
}

func (bot *Bot) newLink() string {
	var random [24]byte
	rand.Read(random[:])
	code := base64.RawURLEncoding.EncodeToString(random[:])

	bot.pairMu.Lock()
	defer bot.pairMu.Unlock()
	bot.pairCode = code
	bot.pairExpires = time.Now().Add(linkLifetime)
	return code
}

func (bot *Bot) redeemLink(ctx context.Context, chatID int64, code string) (bool, error) {
	bot.pairMu.Lock()
	defer bot.pairMu.Unlock()
	if bot.pairCode == "" || !time.Now().Before(bot.pairExpires) ||
		subtle.ConstantTimeCompare([]byte(code), []byte(bot.pairCode)) != 1 {
		return false, nil
	}
	if err := bot.ledger.LinkTelegramChat(ctx, bot.owner, chatID); err != nil {
		return false, err
	}
	bot.pairCode = ""
	return true, nil
}
