package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
)

type userSeed struct {
	Email    string
	Password string
	Name     string
}

type authResponse struct {
	Tokens struct {
		AccessToken string `json:"access_token"`
	} `json:"tokens"`
}

type tokenPair struct {
	AccessToken string
}

type httpError struct {
	StatusCode int
	body       string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("http %d: %s", e.StatusCode, e.body)
}

func main() {
	baseURL := flag.String("base-url", "http://localhost:8080", "API base URL")
	flag.Parse()

	users := []userSeed{
		{Email: "demo1@kidpech.app", Password: "Passw0rd!", Name: "Demo One"},
		{Email: "demo2@kidpech.app", Password: "Passw0rd!", Name: "Demo Two"},
		{Email: "demo3@kidpech.app", Password: "Passw0rd!", Name: "Demo Three"},
	}

	for _, u := range users {
		if err := seedUser(*baseURL, u); err != nil {
			log.Printf("seed %s failed: %v", u.Email, err)
		}
	}
}

func seedUser(baseURL string, user userSeed) error {
	if err := registerUser(baseURL, user); err != nil {
		var httpErr *httpError
		if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusConflict {
			return err
		}
	}

	tokens, err := login(baseURL, user.Email, user.Password)
	if err != nil {
		return fmt.Errorf("login user: %w", err)
	}

	if err := createProfile(baseURL, tokens.AccessToken, user.Name); err != nil {
		return fmt.Errorf("create profile: %w", err)
	}
	return nil
}

func registerUser(baseURL string, user userSeed) error {
	payload := map[string]string{
		"email":    user.Email,
		"password": user.Password,
		"name":     user.Name,
	}
	return postJSON(baseURL+"/api/v1/auth/register", payload, nil)
}

func login(baseURL, email, password string) (*tokenPair, error) {
	payload := map[string]string{"email": email, "password": password}
	resp := authResponse{}
	if err := postJSON(baseURL+"/api/v1/auth/login", payload, &resp); err != nil {
		return nil, err
	}
	return &tokenPair{AccessToken: resp.Tokens.AccessToken}, nil
}

func createProfile(baseURL, token, name string) error {
	payload := map[string]string{
		"first_name": name,
		"last_name":  "Tester",
		"bio":        "Demo profile seeded via script",
	}
	return postJSONWithAuth(baseURL+"/api/v1/profiles", payload, token, nil)
}

func postJSON(url string, payload interface{}, out interface{}) error {
	return doJSONRequest(url, payload, "", out)
}

func postJSONWithAuth(url string, payload interface{}, token string, out interface{}) error {
	return doJSONRequest(url, payload, token, out)
}

func doJSONRequest(url string, payload interface{}, token string, out interface{}) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return &httpError{StatusCode: resp.StatusCode, body: string(b)}
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
