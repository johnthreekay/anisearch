package sonarr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

type Series struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	SortTitle  string `json:"sortTitle"`
	SeriesType string `json:"seriesType"`
	TVDBID     int    `json:"tvdbId"`
	Path       string `json:"path"`
	Statistics struct {
		EpisodeFileCount int `json:"episodeFileCount"`
		EpisodeCount     int `json:"episodeCount"`
		PercentOfEps     float64 `json:"percentOfEpisodes"`
	} `json:"statistics"`
}

type CommandBody struct {
	Name     string `json:"name"`
	SeriesID int    `json:"seriesId,omitempty"`
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	return c.http.Do(req)
}

func (c *Client) GetSeries() ([]Series, error) {
	resp, err := c.doRequest("GET", "/api/v3/series", nil)
	if err != nil {
		return nil, fmt.Errorf("sonarr get series failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sonarr returned %d: %s", resp.StatusCode, string(body))
	}

	var series []Series
	if err := json.NewDecoder(resp.Body).Decode(&series); err != nil {
		return nil, fmt.Errorf("failed to parse sonarr series: %w", err)
	}

	return series, nil
}

func (c *Client) RescanSeries(seriesID int) error {
	cmd := CommandBody{
		Name:     "RescanSeries",
		SeriesID: seriesID,
	}

	resp, err := c.doRequest("POST", "/api/v3/command", cmd)
	if err != nil {
		return fmt.Errorf("sonarr rescan failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sonarr rescan failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) RescanAll() error {
	cmd := CommandBody{
		Name: "RescanSeries",
	}

	resp, err := c.doRequest("POST", "/api/v3/command", cmd)
	if err != nil {
		return fmt.Errorf("sonarr rescan all failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sonarr rescan all failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) TestConnection() error {
	resp, err := c.doRequest("GET", "/api/v3/system/status", nil)
	if err != nil {
		return fmt.Errorf("sonarr connection test failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("sonarr returned status %d", resp.StatusCode)
	}
	return nil
}
