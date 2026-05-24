package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const clientID = "58e45408-fb06-4e5a-b151-65e1796882a2"

const redirectURI = "http://localhost:8080/callback"

type Account struct {
	Username     string
	UUID         string
	AccessToken  string
	RefreshToken string
}

type microsoftTokenResponse struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
}

type xboxAuthResponse struct {
	IssueInstant  string `json:"IssueInstant"`
	NotAfter      string `json:"NotAfter"`
	Token         string `json:"Token"`
	DisplayClaims struct {
		XUI []struct {
			UHS string `json:"uhs"`
		} `json:"xui"`
	} `json:"DisplayClaims"`
}

type minecraftLoginResponse struct {
	Username    string `json:"username"`
	Roles       []any  `json:"roles"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type minecraftProfileResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Skins          []any  `json:"skins"`
	Capes          []any  `json:"capes"`
	ProfileActions []any  `json:"profileActions"`
}

func LoginMicrosoftLoopback() (*Account, error) {
	id := strings.TrimSpace(clientID)
	if id == "" || id == "PASTE_YOUR_CLIENT_ID_HERE" {
		return nil, fmt.Errorf("Microsoft Client ID není nastavený v internal/auth/auth.go")
	}

	verifier, err := randomBase64URL(64)
	if err != nil {
		return nil, err
	}

	state, err := randomBase64URL(32)
	if err != nil {
		return nil, err
	}

	challenge := pkceChallenge(verifier)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	code, err := StartLoopback(ctx, redirectURI, func() string {
		return microsoftAuthURL(id, redirectURI, state, challenge)
	}, state)
	if err != nil {
		return nil, err
	}

	msToken, err := exchangeMicrosoftCode(id, redirectURI, code, verifier)
	if err != nil {
		return nil, err
	}

	xbl, err := authenticateXboxLive(msToken.AccessToken)
	if err != nil {
		return nil, err
	}

	xsts, err := authorizeXSTS(xbl.Token)
	if err != nil {
		return nil, err
	}

	if len(xsts.DisplayClaims.XUI) == 0 || xsts.DisplayClaims.XUI[0].UHS == "" {
		return nil, fmt.Errorf("XSTS response neobsahuje UHS")
	}

	uhs := xsts.DisplayClaims.XUI[0].UHS

	mcToken, err := loginMinecraftWithXbox(uhs, xsts.Token)
	if err != nil {
		return nil, err
	}

	profile, err := getMinecraftProfile(mcToken.AccessToken)
	if err != nil {
		return nil, err
	}

	if profile.ID == "" || profile.Name == "" {
		return nil, fmt.Errorf("Minecraft účet nemá koupený/validní profil")
	}

	return &Account{
		Username:     profile.Name,
		UUID:         formatUUID(profile.ID),
		AccessToken:  mcToken.AccessToken,
		RefreshToken: msToken.RefreshToken,
	}, nil
}

func microsoftAuthURL(clientID string, redirectURI string, state string, codeChallenge string) string {
	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("response_type", "code")
	v.Set("redirect_uri", redirectURI)
	v.Set("scope", "XboxLive.signin offline_access")
	v.Set("state", state)
	v.Set("code_challenge", codeChallenge)
	v.Set("code_challenge_method", "S256")
	v.Set("prompt", "select_account")

	return "https://login.live.com/oauth20_authorize.srf?" + v.Encode()
}

func exchangeMicrosoftCode(clientID string, redirectURI string, code string, verifier string) (*microsoftTokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", verifier)

	req, err := http.NewRequest(http.MethodPost, "https://login.live.com/oauth20_token.srf", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var out microsoftTokenResponse
	if err := doJSON(req, &out); err != nil {
		return nil, err
	}

	if out.AccessToken == "" {
		return nil, fmt.Errorf("Microsoft token response neobsahuje access_token")
	}

	return &out, nil
}

func authenticateXboxLive(msAccessToken string) (*xboxAuthResponse, error) {
	body := map[string]any{
		"Properties": map[string]any{
			"AuthMethod": "RPS",
			"SiteName":   "user.auth.xboxlive.com",
			"RpsTicket":  "d=" + msAccessToken,
		},
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
	}

	req, err := jsonRequest("https://user.auth.xboxlive.com/user/authenticate", body, "")
	if err != nil {
		return nil, err
	}

	var out xboxAuthResponse
	if err := doJSON(req, &out); err != nil {
		return nil, err
	}

	if out.Token == "" {
		return nil, fmt.Errorf("Xbox Live response neobsahuje token")
	}

	return &out, nil
}

func authorizeXSTS(xblToken string) (*xboxAuthResponse, error) {
	body := map[string]any{
		"Properties": map[string]any{
			"SandboxId":  "RETAIL",
			"UserTokens": []string{xblToken},
		},
		"RelyingParty": "rp://api.minecraftservices.com/",
		"TokenType":    "JWT",
	}

	req, err := jsonRequest("https://xsts.auth.xboxlive.com/xsts/authorize", body, "")
	if err != nil {
		return nil, err
	}

	var out xboxAuthResponse
	if err := doJSON(req, &out); err != nil {
		return nil, err
	}

	if out.Token == "" {
		return nil, fmt.Errorf("XSTS response neobsahuje token")
	}

	return &out, nil
}

func loginMinecraftWithXbox(uhs string, xstsToken string) (*minecraftLoginResponse, error) {
	body := map[string]any{
		"identityToken": "XBL3.0 x=" + uhs + ";" + xstsToken,
	}

	req, err := jsonRequest("https://api.minecraftservices.com/authentication/login_with_xbox", body, "")
	if err != nil {
		return nil, err
	}

	var out minecraftLoginResponse
	if err := doJSON(req, &out); err != nil {
		return nil, err
	}

	if out.AccessToken == "" {
		return nil, fmt.Errorf("Minecraft login response neobsahuje access_token")
	}

	return &out, nil
}

func getMinecraftProfile(accessToken string) (*minecraftProfileResponse, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.minecraftservices.com/minecraft/profile", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	var out minecraftProfileResponse
	if err := doJSON(req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func jsonRequest(endpoint string, body any, bearer string) (*http.Request, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	return req, nil
}

func doJSON(req *http.Request, out any) error {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	if len(data) == 0 {
		return fmt.Errorf("prázdná odpověď")
	}

	return json.Unmarshal(data, out)
}

func randomBase64URL(size int) (string, error) {
	b := make([]byte, size)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func formatUUID(id string) string {
	id = strings.ReplaceAll(id, "-", "")

	if len(id) != 32 {
		return id
	}

	return fmt.Sprintf("%s-%s-%s-%s-%s",
		id[0:8],
		id[8:12],
		id[12:16],
		id[16:20],
		id[20:32],
	)
}
