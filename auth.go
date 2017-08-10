package chapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"golang.org/x/oauth2"
)

// initCaAuth starts the authentication process for oAuth to Channel Advisor.
func initCaAuth(ctx context.Context) (*oauth2.Token, *oauth2.Config, error) {
	clID := os.Getenv("CLIENT_ID")
	clS := os.Getenv("CLIENT_SECRET")
	RedURI := os.Getenv("REDIRECT_URL")
	if clID == "" || clS == "" || RedURI == "" {
		return nil, nil, errors.New("No environment variables CLIENT_ID, CLIENT_SECRET, or REDIRECT_URL")
	}

	conf := &oauth2.Config{
		ClientID:     clID,
		ClientSecret: clS,
		Scopes:       []string{"inventory"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://api.channeladvisor.com/oauth2/authorize",
			TokenURL: "https://api.channeladvisor.com/oauth2/token",
		},
		RedirectURL: RedURI,
	}
	cacheFile := "credentials/ca-toks.json"
	tok, err := tokenFromFile(cacheFile)
	if err != nil {
		tok = getTokenFromWeb(ctx, conf)
		saveToken(cacheFile, tok)
	}
	return tok, conf, nil
}

// getTokenFromWeb authenticate user and use code to stor a token on server.
func getTokenFromWeb(ctx context.Context, conf *oauth2.Config) *oauth2.Token {
	url := conf.AuthCodeURL("state", oauth2.AccessTypeOffline)
	fmt.Printf("Visit the URL for the auth dialog: %v", url)
	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Fatal(err)
	}

	tok, err := conf.Exchange(ctx, code)
	if err != nil {
		log.Fatal(err)
	}
	return tok
}

// tokenFromFile open file where token is saved.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	t := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(t)
	defer f.Close()
	return t, err
}

// saveToken uses a file path to create a file and store the
// token in it.
func saveToken(file string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", file)
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
