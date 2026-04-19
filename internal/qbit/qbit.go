package qbit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL  string
	user     string
	pass     string
	category string
	http     *http.Client
	loggedIn bool
}

type Torrent struct {
	Hash     string  `json:"hash"`
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Progress float64 `json:"progress"`
	DlSpeed  int64   `json:"dlspeed"`
	State    string  `json:"state"`
	Category string  `json:"category"`
	ETA      int64   `json:"eta"`
}

func NewClient(baseURL, user, pass, category string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		user:     user,
		pass:     pass,
		category: category,
		http: &http.Client{
			Timeout: 10 * time.Second,
			Jar:     jar,
		},
	}
}

func (c *Client) Login() error {
	data := url.Values{
		"username": {c.user},
		"password": {c.pass},
	}

	resp, err := c.http.PostForm(c.baseURL+"/api/v2/auth/login", data)
	if err != nil {
		return fmt.Errorf("qbit login failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(body) == "Fails." {
		return fmt.Errorf("qbit auth failed (status %d)", resp.StatusCode)
	}

	c.loggedIn = true
	return nil
}

func (c *Client) ensureLogin() error {
	if !c.loggedIn {
		return c.Login()
	}
	return nil
}

// doGet performs a GET with automatic re-auth on 403.
func (c *Client) doGet(endpoint string) (*http.Response, error) {
	if err := c.ensureLogin(); err != nil {
		return nil, err
	}

	resp, err := c.http.Get(c.baseURL + endpoint)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusForbidden {
		resp.Body.Close()
		c.loggedIn = false
		if err := c.Login(); err != nil {
			return nil, fmt.Errorf("re-auth failed: %w", err)
		}
		resp, err = c.http.Get(c.baseURL + endpoint)
		if err != nil {
			return nil, err
		}
	}

	return resp, nil
}

// doPostForm performs a POST with form data and automatic re-auth on 403.
func (c *Client) doPostForm(endpoint string, data url.Values) (*http.Response, error) {
	if err := c.ensureLogin(); err != nil {
		return nil, err
	}

	resp, err := c.http.PostForm(c.baseURL+endpoint, data)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusForbidden {
		resp.Body.Close()
		c.loggedIn = false
		if err := c.Login(); err != nil {
			return nil, fmt.Errorf("re-auth failed: %w", err)
		}
		resp, err = c.http.PostForm(c.baseURL+endpoint, data)
		if err != nil {
			return nil, err
		}
	}

	return resp, nil
}

func (c *Client) AddMagnet(magnet string) error {
	data := url.Values{
		"urls":     {magnet},
		"category": {c.category},
	}

	resp, err := c.doPostForm("/api/v2/torrents/add", data)
	if err != nil {
		return fmt.Errorf("failed to add torrent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qbit add failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) AddTorrentURL(torrentURL string) error {
	data := url.Values{
		"urls":     {torrentURL},
		"category": {c.category},
	}

	resp, err := c.doPostForm("/api/v2/torrents/add", data)
	if err != nil {
		return fmt.Errorf("failed to add torrent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qbit add failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) GetTorrents() ([]Torrent, error) {
	params := url.Values{}
	if c.category != "" {
		params.Set("category", c.category)
	}

	resp, err := c.doGet("/api/v2/torrents/info?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("failed to get torrents: %w", err)
	}
	defer resp.Body.Close()

	var torrents []Torrent
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return nil, fmt.Errorf("failed to parse torrents: %w", err)
	}

	return torrents, nil
}

func (c *Client) GetTorrent(hash string) (*Torrent, error) {
	torrents, err := c.GetTorrents()
	if err != nil {
		return nil, err
	}

	hash = strings.ToLower(hash)
	for _, t := range torrents {
		if strings.ToLower(t.Hash) == hash {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("torrent %s not found", hash)
}

func (c *Client) TestConnection() error {
	resp, err := c.doGet("/api/v2/app/version")
	if err != nil {
		return fmt.Errorf("qbit connection test failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("qbit returned status %d", resp.StatusCode)
	}
	return nil
}
