package scoutbook

import (
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

func (c *Client) FetchRoster(ctx context.Context, memberType MemberType) ([]RosterMember, error) {
	if memberType != EndpointUnitAdults && memberType != EndpointUnitYouths {
		return nil, fmt.Errorf("invalid member type: %s", memberType)
	}

	url := fmt.Sprintf("%s/organizations/v2/units/%s/%s", c.baseURL, c.orgGUID, memberType)

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

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("unexpected status: %d, body: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("[sync] FetchRoster %s response (%d bytes): %s", memberType, len(bodyBytes), string(bodyBytes))

	var roster unitRosterResponse
	if err := json.Unmarshal(bodyBytes, &roster); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	for _, m := range roster.Users {
		log.Printf("[sync]   roster member: memberId=%s name=%s %s personGuid=%s email=%s",
			m.MemberID, m.FirstName, m.LastName, m.PersonGUID, m.Email)
	}

	return roster.Users, nil
}

func (c *Client) OrgGUID() string {
	return c.orgGUID
}

func (c *Client) SetToken(token string) {
	c.token = token
}

func (c *Client) SetOrgGUID(orgGUID string) {
	c.orgGUID = orgGUID
}
