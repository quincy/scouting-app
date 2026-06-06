package scoutbook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type Client struct {
	baseURL string
	token   string
	orgGUID string
	http    *http.Client
}

func NewClient(baseURL, token, orgGUID string) *Client {
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
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("unexpected status: %d, body: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("[sync] FetchRoster %s response (%d bytes): %s", memberType, len(bodyBytes), string(bodyBytes))

	var roster rosterResponse
	if err := json.Unmarshal(bodyBytes, &roster); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	for _, m := range roster.Data {
		log.Printf("[sync]   roster member: memberId=%s name=%s %s personGuid=%s", m.MemberID, m.FirstName, m.LastName, m.PersonGUID)
	}

	return roster.Data, nil
}

func (c *Client) OrgGUID() string {
	return c.orgGUID
}

func (c *Client) SetToken(token string) {
	c.token = token
}

func (c *Client) FetchProfile(ctx context.Context, personGUID string) (*PersonProfile, error) {
	url := fmt.Sprintf("%s/persons/v2/%s/personprofile", c.baseURL, personGUID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("unexpected status: %d, body: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("[sync] FetchProfile %s response (%d bytes): %s", personGUID, len(bodyBytes), string(bodyBytes))

	var profile PersonProfile
	if err := json.Unmarshal(bodyBytes, &profile); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &profile, nil
}
