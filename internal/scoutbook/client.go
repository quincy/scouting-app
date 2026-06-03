package scoutbook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const defaultBaseURL = "https://advancements.scouting.org"

type Client struct {
	baseURL string
	token   string
	orgGUID string
	http    *http.Client
}

func NewClient(baseURL, token, orgGUID string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL: baseURL,
		token:   token,
		orgGUID: orgGUID,
		http:    http.DefaultClient,
	}
}

type rosterRequest struct {
	IncludeRegistrationDetails bool `json:"includeRegistrationDetails"`
	IncludeExpired             bool `json:"includeExpired"`
}

func (c *Client) FetchRoster(ctx context.Context, memberType MemberType) ([]RosterMember, error) {
	if memberType != EndpointAdults && memberType != EndpointYouths {
		return nil, fmt.Errorf("invalid member type: %s", memberType)
	}

	url := fmt.Sprintf("%s/organizations/v2/%s/%s", c.baseURL, c.orgGUID, memberType)
	body := rosterRequest{
		IncludeRegistrationDetails: true,
		IncludeExpired:             true,
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, fmt.Errorf("encode request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var roster rosterResponse
	if err := json.NewDecoder(resp.Body).Decode(&roster); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return roster.Data, nil
}

func (c *Client) FetchProfile(ctx context.Context, personGUID string) (*PersonProfile, error) {
	url := fmt.Sprintf("%s/persons/v2/%s/personprofile", c.baseURL, personGUID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var profile PersonProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &profile, nil
}
