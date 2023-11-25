package files

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/alioygur/gores"
	"github.com/bwmarrin/discordgo"
	"golang.org/x/oauth2"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
var discord *discordgo.Session

// Return a random character sequence of n length
func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

const (
	authURL      string = "https://discordapp.com/api/oauth2/authorize"
	tokenURL     string = "https://discordapp.com/api/oauth2/token"
	userEndpoint string = "https://discordapp.com/api/v9/users/@me"
)

var cachedConfig *oauth2.Config

type DiscordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"string"`
	Email         string `json:"email"`
	Verified      bool   `json:"verified"`
}

func (h *HTTPService) discordGetLogoutRoute(w http.ResponseWriter, r *http.Request) {
	session := h.getSession(w, r)
	if session == nil {
		return
	}

	session.Values["discord-user-id"] = nil
	session.Values["discord-state"] = nil
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (h *HTTPService) discordGetLoginRoute(w http.ResponseWriter, r *http.Request) {
	session := h.getSession(w, r)
	if session == nil {
		return
	}

	err := r.ParseForm()
	if err != nil {
		gores.Error(w, http.StatusBadRequest, "Bad Form Data")
		return
	}

	state := randSeq(32)
	session.Values["discord-state"] = state
	session.Save(r, w)

	url := cachedConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *HTTPService) discordGetLoginCallbackRoute(w http.ResponseWriter, r *http.Request) {
	session := h.getSession(w, r)
	if session == nil {
		return
	}

	state := r.FormValue("state")
	if state != session.Values["discord-state"] {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	errorMessage := r.FormValue("error")
	if errorMessage != "" {
		gores.Error(w, http.StatusBadRequest, fmt.Sprintf("Error: %v", errorMessage))
		return
	}

	token, err := cachedConfig.Exchange(context.Background(), r.FormValue("code"))
	if err != nil {
		gores.Error(w, 500, fmt.Sprintf("Error: %v", err))
		return
	}

	req, err := http.NewRequest("GET", userEndpoint, nil)
	if err != nil {
		gores.Error(w, 500, fmt.Sprintf("Error: %v", err))
		return
	}

	req.Header.Set("Authorization", token.Type()+" "+token.AccessToken)
	client := &http.Client{Timeout: 10 * time.Second}

	res, err := client.Do(req)
	if err != nil {
		gores.Error(w, 500, fmt.Sprintf("Error: %v", err))
		return
	}

	if res.StatusCode != 200 {
		log.Printf("ERROR: %v", res.StatusCode)
		body, err := io.ReadAll(res.Body)
		if err != nil {
			gores.Error(w, 500, fmt.Sprintf("Error reading discord user metadata: %v", err))
			return
		}

		log.Printf("body: %s", body)
		gores.Error(w, 500, "Failed to read discord user metadata")
		return
	}

	var discordUser DiscordUser
	err = json.NewDecoder(res.Body).Decode(&discordUser)
	if err != nil {
		gores.Error(w, 500, fmt.Sprintf("Error: %v", err))
		return
	}

	session.Values["discord-user-id"] = discordUser.ID
	session.Values["discord-state"] = nil
	err = session.Save(r, w)
	if err != nil {
		panic(err)
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func setupDiscord(config *DiscordConfig, baseURL string) {
	s, err := discordgo.New(config.Token)
	if err != nil {
		panic(err)
	}
	discord = s

	cachedConfig = &oauth2.Config{
		ClientID:     config.ClientId,
		ClientSecret: config.ClientSecret,
		RedirectURL:  baseURL + "/discord/login/callback",
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
		Scopes: []string{"identify"},
	}
}
