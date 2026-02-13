package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/MelianLabs/litetracker-mcp/internal/config"
)

var client = &http.Client{Timeout: 30 * time.Second}

func request(method, path string, body io.Reader) (*http.Response, error) {
	u := config.C.BaseURL + path
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-TrackerToken", config.C.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LiteTracker API %d: %s — %s", resp.StatusCode, resp.Status, string(b))
	}
	return resp, nil
}

func decode[T any](resp *http.Response) (T, error) {
	var result T
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return result, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}

func GetMe() (Me, error) {
	resp, err := request("GET", "/me", nil)
	if err != nil {
		return Me{}, err
	}
	return decode[Me](resp)
}

func ListProjects() ([]Project, error) {
	resp, err := request("GET", "/projects", nil)
	if err != nil {
		return nil, err
	}
	return decode[[]Project](resp)
}

func ListStories(projectID int, opts ListStoriesOpts) ([]Story, error) {
	params := url.Values{}
	if opts.Filter != "" {
		params.Set("filter", opts.Filter)
	}
	if opts.Query != 0 {
		params.Set("query", strconv.Itoa(opts.Query))
	}
	if opts.Owners != 0 {
		params.Set("owners", strconv.Itoa(opts.Owners))
	}
	if opts.SectionType != "" {
		params.Set("section_type", opts.SectionType)
	}
	if opts.OwnedBy != 0 {
		params.Set("owned_by", strconv.Itoa(opts.OwnedBy))
	}
	if opts.State != "" {
		params.Set("with_state", opts.State)
	}
	limit := opts.Limit
	if limit == 0 {
		limit = 20
	}
	params.Set("limit", strconv.Itoa(limit))

	resp, err := request("GET", fmt.Sprintf("/projects/%d/stories?%s", projectID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}
	return decode[[]Story](resp)
}

func GetStory(projectID, storyID int) (Story, error) {
	resp, err := request("GET", fmt.Sprintf("/projects/%d/stories/%d", projectID, storyID), nil)
	if err != nil {
		return Story{}, err
	}
	return decode[Story](resp)
}

func GetStoryComments(projectID, storyID int) ([]Comment, error) {
	resp, err := request("GET", fmt.Sprintf("/projects/%d/stories/%d/comments", projectID, storyID), nil)
	if err != nil {
		return nil, err
	}
	return decode[[]Comment](resp)
}

func PostComment(projectID, storyID int, text string) (Comment, error) {
	payload, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return Comment{}, fmt.Errorf("marshal comment: %w", err)
	}
	body := strings.NewReader(string(payload))
	resp, err := request("POST", fmt.Sprintf("/projects/%d/stories/%d/comments", projectID, storyID), body)
	if err != nil {
		return Comment{}, err
	}
	return decode[Comment](resp)
}

func CreateStory(projectID int, params map[string]any) (Story, error) {
	payload, err := json.Marshal(params)
	if err != nil {
		return Story{}, fmt.Errorf("marshal story: %w", err)
	}
	body := strings.NewReader(string(payload))
	resp, err := request("POST", fmt.Sprintf("/projects/%d/stories", projectID), body)
	if err != nil {
		return Story{}, err
	}
	return decode[Story](resp)
}

func GetProjectActivity(projectID int, occurredAfter string) ([]Activity, error) {
	params := url.Values{}
	params.Set("occurred_after", occurredAfter)
	params.Set("limit", "100")
	resp, err := request("GET", fmt.Sprintf("/projects/%d/activity?%s", projectID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}
	return decode[[]Activity](resp)
}
