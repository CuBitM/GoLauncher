package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	ClientID    = "00000000402b5328"
	RedirectURI = "https://login.live.com/oauth20_desktop.srf"
	AuthURL     = "https://login.live.com/oauth20_authorize.srf"
	TokenURL    = "https://login.live.com/oauth20_token.srf"

	XBLAuthURL   = "https://user.auth.xboxlive.com/user/authenticate"
	XSTSAuthURL  = "https://xsts.auth.xboxlive.com/xsts/authorize"
	MCAuthURL    = "https://api.minecraftservices.com/authentication/login_with_xbox"
	MCProfileURL = "https://api.minecraftservices.com/minecraft/profile"
)

type Account struct {
	Username    string    `json:"username"`
	UUID        string    `json:"uuid"`
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type MSAuthFlow struct {
	client *http.Client
}

func NewMSAuthFlow() *MSAuthFlow {
	return &MSAuthFlow{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *MSAuthFlow) GetAuthURL() string {
	params := url.Values{
		"client_id":     {ClientID},
		"response_type": {"code"},
		"redirect_uri":  {RedirectURI},
		"scope":         {"XboxLive.signin offline_access"},
	}
	return AuthURL + "?" + params.Encode()
}

func (a *MSAuthFlow) ExchangeCode(code string) (*Account, error) {
	msToken, err := a.getMSToken(code)
	if err != nil {
		return nil, fmt.Errorf("MS token error: %w", err)
	}

	xblToken, userHash, err := a.getXBLToken(msToken)
	if err != nil {
		return nil, fmt.Errorf("XBL token error: %w", err)
	}

	xstsToken, err := a.getXSTSToken(xblToken)
	if err != nil {
		return nil, fmt.Errorf("XSTS token error: %w", err)
	}

	mcToken, err := a.getMCToken(xstsToken, userHash)
	if err != nil {
		return nil, fmt.Errorf("MC token error: %w", err)
	}

	profile, err := a.getProfile(mcToken)
	if err != nil {
		return nil, fmt.Errorf("profile error: %w", err)
	}

	return &Account{
		Username:    profile["name"].(string),
		UUID:        profile["id"].(string),
		AccessToken: mcToken,
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}, nil
}

func (a *MSAuthFlow) getMSToken(code string) (string, error) {
	data := url.Values{
		"client_id":    {ClientID},
		"code":         {code},
		"grant_type":   {"authorization_code"},
		"redirect_uri": {RedirectURI},
	}

	resp, err := a.client.PostForm(TokenURL, data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	token, ok := result["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("no access_token in response")
	}
	return token, nil
}

func (a *MSAuthFlow) getXBLToken(msToken string) (string, string, error) {
	payload := map[string]interface{}{
		"Properties": map[string]interface{}{
			"AuthMethod": "RPS",
			"SiteName":   "user.auth.xboxlive.com",
			"RpsTicket":  "d=" + msToken,
		},
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", XBLAuthURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	token := result["Token"].(string)
	claims := result["DisplayClaims"].(map[string]interface{})
	xui := claims["xui"].([]interface{})
	userHash := xui[0].(map[string]interface{})["uhs"].(string)

	return token, userHash, nil
}

func (a *MSAuthFlow) getXSTSToken(xblToken string) (string, error) {
	payload := map[string]interface{}{
		"Properties": map[string]interface{}{
			"SandboxId":  "RETAIL",
			"UserTokens": []string{xblToken},
		},
		"RelyingParty": "rp://api.minecraftservices.com/",
		"TokenType":    "JWT",
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", XSTSAuthURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result["Token"].(string), nil
}

func (a *MSAuthFlow) getMCToken(xstsToken, userHash string) (string, error) {
	payload := map[string]interface{}{
		"identityToken": fmt.Sprintf("XBL3.0 x=%s;%s", userHash, xstsToken),
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", MCAuthURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result["access_token"].(string), nil
}

func (a *MSAuthFlow) getProfile(mcToken string) (map[string]interface{}, error) {
	req, _ := http.NewRequest("GET", MCProfileURL, nil)
	req.Header.Set("Authorization", "Bearer "+mcToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result, nil
}

func (a *MSAuthFlow) IsValid(account *Account) bool {
	if account == nil {
		return false
	}
	return time.Now().Before(account.ExpiresAt) && account.AccessToken != ""
}

func buildIdentityToken(userHash, xstsToken string) string {
	return strings.Join([]string{"XBL3.0 x=" + userHash, xstsToken}, ";")
}
