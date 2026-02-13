package api

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MelianLabs/litetracker-mcp/internal/config"
)

var (
	webOnce   sync.Once
	webClient *WebClient
)

type WebClient struct {
	mu       sync.Mutex
	client   *http.Client
	loggedIn bool
}

func getWebClient() *WebClient {
	webOnce.Do(func() {
		jar, _ := cookiejar.New(nil)
		webClient = &WebClient{
			client: &http.Client{
				Timeout: 30 * time.Second,
				Jar:     jar,
			},
		}
	})
	return webClient
}

var csrfRegex = regexp.MustCompile(`csrf-token[^>]*content="([^"]*)"`)

func (wc *WebClient) ensureLoggedIn() error {
	if wc.loggedIn {
		return nil
	}
	if config.C.Email == "" || config.C.Password == "" {
		return fmt.Errorf("LITETRACKER_EMAIL and LITETRACKER_PASSWORD must be set in ~/litetracker-go/.env for posting comments (LiteTracker API does not support comment creation)")
	}

	// GET /login to get CSRF token and session cookie
	loginURL := config.C.WebURL + "/login"
	resp, err := wc.client.Get(loginURL)
	if err != nil {
		return fmt.Errorf("fetch login page: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	matches := csrfRegex.FindSubmatch(body)
	if len(matches) < 2 {
		return fmt.Errorf("could not find CSRF token on login page")
	}
	csrfToken := string(matches[1])

	// POST /login with form data
	form := url.Values{
		"authenticity_token": {csrfToken},
		"user[login]":       {config.C.Email},
		"user[password]":    {config.C.Password},
		"user[remember_me]": {"1"},
	}
	req, err := http.NewRequest("POST", loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")

	resp, err = wc.client.Do(req)
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode == 422 || resp.StatusCode == 401 {
		return fmt.Errorf("login failed (status %d): check LITETRACKER_EMAIL and LITETRACKER_PASSWORD in ~/litetracker-go/.env", resp.StatusCode)
	}

	wc.loggedIn = true
	return nil
}

// apiV1Comment represents the /api/v1 comment response structure
type apiV1Comment struct {
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			Content   string `json:"content"`
			CreatedAt string `json:"created-at"`
			UserID    int    `json:"user-id"`
		} `json:"attributes"`
	} `json:"data"`
}

func (wc *WebClient) postComment(storyID int, text string) (Comment, error) {
	commentURL := fmt.Sprintf("%s/api/v1/stories/%d/comments", config.C.WebURL, storyID)

	// Build multipart form data (matches the SPA's FormData format)
	var buf strings.Builder
	w := multipart.NewWriter(&buf)
	w.WriteField("comment[content]", text)
	w.WriteField("comment[user_id]", strconv.Itoa(config.C.UserID))
	w.WriteField("comment[commentable_type]", "Story")
	w.WriteField("comment[commentable_id]", strconv.Itoa(storyID))
	w.Close()

	req, err := http.NewRequest("POST", commentURL, strings.NewReader(buf.String()))
	if err != nil {
		return Comment{}, fmt.Errorf("build comment request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := wc.client.Do(req)
	if err != nil {
		return Comment{}, fmt.Errorf("post comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return Comment{}, fmt.Errorf("post comment failed (status %d): %s", resp.StatusCode, string(b))
	}

	body, _ := io.ReadAll(resp.Body)
	var result apiV1Comment
	if err := json.Unmarshal(body, &result); err != nil {
		return Comment{Text: text}, nil
	}

	id, _ := strconv.Atoi(result.Data.ID)
	return Comment{
		ID:        id,
		Text:      result.Data.Attributes.Content,
		PersonID:  result.Data.Attributes.UserID,
		CreatedAt: result.Data.Attributes.CreatedAt,
	}, nil
}

func (wc *WebClient) addLabel(storyID, projectID int, name string) (Label, error) {
	labelURL := fmt.Sprintf("%s/api/v1/stories/%d/labels", config.C.WebURL, storyID)
	payload, _ := json.Marshal(map[string]any{
		"label": map[string]any{"name": name, "project_id": projectID},
	})
	req, err := http.NewRequest("POST", labelURL, strings.NewReader(string(payload)))
	if err != nil {
		return Label{}, fmt.Errorf("build label request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := wc.client.Do(req)
	if err != nil {
		return Label{}, fmt.Errorf("add label: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return Label{}, fmt.Errorf("add label failed (status %d): %s", resp.StatusCode, string(b))
	}

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				Name string `json:"name"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return Label{Name: name}, nil
	}
	id, _ := strconv.Atoi(result.Data.ID)
	return Label{ID: id, Name: result.Data.Attributes.Name}, nil
}

func WebAddLabel(projectID, storyID int, name string) (Label, error) {
	wc := getWebClient()
	wc.mu.Lock()
	defer wc.mu.Unlock()

	if err := wc.ensureLoggedIn(); err != nil {
		return Label{}, err
	}

	label, err := wc.addLabel(storyID, projectID, name)
	if err != nil && (strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "sign in")) {
		wc.loggedIn = false
		if err := wc.ensureLoggedIn(); err != nil {
			return Label{}, err
		}
		return wc.addLabel(storyID, projectID, name)
	}
	return label, err
}

func (wc *WebClient) addOwner(storyID, projectID, ownerID int) ([]StoryOwner, error) {
	// Use v5 API (token auth, always reliable) to get current owners
	story, err := GetStory(projectID, storyID)
	if err != nil {
		return nil, fmt.Errorf("fetch story owners: %w", err)
	}

	// Build owner_ids list from story.Owners (v5 API populates Owners, not OwnerIDs)
	ids := make([]int, 0, len(story.Owners)+1)
	for _, o := range story.Owners {
		if o.UserID == ownerID {
			// Already an owner — return current owners
			return story.Owners, nil
		}
		ids = append(ids, o.UserID)
	}
	ids = append(ids, ownerID)

	// PUT story update with new owner_ids via internal API
	storyURL := fmt.Sprintf("%s/api/v1/stories/%d", config.C.WebURL, storyID)
	payload, _ := json.Marshal(map[string]any{
		"story": map[string]any{"owner_ids": ids},
	})
	req, err := http.NewRequest("PUT", storyURL, strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("build owner request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := wc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("add owner: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("add owner failed (status %d): %s", resp.StatusCode, string(b))
	}

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Owners []StoryOwner `json:"owners"`
	}
	json.Unmarshal(body, &result)
	return result.Owners, nil
}

func WebAddOwner(projectID, storyID, ownerID int) ([]StoryOwner, error) {
	wc := getWebClient()
	wc.mu.Lock()
	defer wc.mu.Unlock()

	if err := wc.ensureLoggedIn(); err != nil {
		return nil, err
	}

	owners, err := wc.addOwner(storyID, projectID, ownerID)
	if err != nil && (strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "sign in")) {
		wc.loggedIn = false
		if err := wc.ensureLoggedIn(); err != nil {
			return nil, err
		}
		return wc.addOwner(storyID, projectID, ownerID)
	}
	return owners, err
}

func WebPostComment(projectID, storyID int, text string) (Comment, error) {
	wc := getWebClient()
	wc.mu.Lock()
	defer wc.mu.Unlock()

	if err := wc.ensureLoggedIn(); err != nil {
		return Comment{}, err
	}

	comment, err := wc.postComment(storyID, text)
	if err != nil && (strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "sign in")) {
		// Session expired, re-login and retry
		wc.loggedIn = false
		if err := wc.ensureLoggedIn(); err != nil {
			return Comment{}, err
		}
		return wc.postComment(storyID, text)
	}
	return comment, err
}
