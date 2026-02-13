package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MelianLabs/litetracker-mcp/internal/api"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func NewServer() *server.MCPServer {
	s := server.NewMCPServer("litetracker", "2.0.0",
		server.WithToolCapabilities(false),
	)

	s.AddTool(mcp.NewTool("get_me",
		mcp.WithDescription("Get current authenticated user info"),
		mcp.WithTitleAnnotation("My Profile"),
	), handleGetMe)

	s.AddTool(mcp.NewTool("list_projects",
		mcp.WithDescription("List all LiteTracker projects"),
		mcp.WithTitleAnnotation("List Projects"),
	), handleListProjects)

	s.AddTool(mcp.NewTool("list_stories",
		mcp.WithDescription("List stories in a LiteTracker project"),
		mcp.WithTitleAnnotation("List Stories"),
		mcp.WithNumber("project_id",
			mcp.Description("LiteTracker project ID"),
			mcp.Required(),
		),
		mcp.WithString("filter",
			mcp.Description("Tracker search syntax, e.g. \"state:started label:bug\""),
		),
		mcp.WithNumber("query",
			mcp.Description("Raw query parameter for searching stories"),
		),
		mcp.WithNumber("owners",
			mcp.Description("Filter by owner user ID (e.g. 568 for Robert)"),
		),
		mcp.WithString("section_type",
			mcp.Description("Section type filter (e.g. user_stories)"),
		),
		mcp.WithNumber("owned_by",
			mcp.Description("Filter by owner user ID (e.g. 568 for Robert)"),
		),
		mcp.WithString("state",
			mcp.Description("Filter by state: started, unstarted, delivered, accepted, rejected"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max stories to return (default 20)"),
		),
	), handleListStories)

	s.AddTool(mcp.NewTool("get_story",
		mcp.WithDescription("Get a single story with its comments"),
		mcp.WithTitleAnnotation("Show Story"),
		mcp.WithNumber("project_id",
			mcp.Description("LiteTracker project ID"),
			mcp.Required(),
		),
		mcp.WithNumber("story_id",
			mcp.Description("Story ID"),
			mcp.Required(),
		),
	), handleGetStory)

	s.AddTool(mcp.NewTool("get_story_comments",
		mcp.WithDescription("Get comments for a story"),
		mcp.WithTitleAnnotation("Show Comments"),
		mcp.WithNumber("project_id",
			mcp.Description("LiteTracker project ID"),
			mcp.Required(),
		),
		mcp.WithNumber("story_id",
			mcp.Description("Story ID"),
			mcp.Required(),
		),
	), handleGetStoryComments)

	s.AddTool(mcp.NewTool("post_comment",
		mcp.WithDescription("Post a comment on a story"),
		mcp.WithTitleAnnotation("Post Comment"),
		mcp.WithNumber("project_id",
			mcp.Description("LiteTracker project ID"),
			mcp.Required(),
		),
		mcp.WithNumber("story_id",
			mcp.Description("Story ID"),
			mcp.Required(),
		),
		mcp.WithString("text",
			mcp.Description("Comment text to post"),
			mcp.Required(),
		),
	), handlePostComment)

	s.AddTool(mcp.NewTool("create_story",
		mcp.WithDescription("Create a new story in a LiteTracker project"),
		mcp.WithTitleAnnotation("Create Story"),
		mcp.WithNumber("project_id",
			mcp.Description("LiteTracker project ID"),
			mcp.Required(),
		),
		mcp.WithString("title",
			mcp.Description("Story title"),
			mcp.Required(),
		),
		mcp.WithString("description",
			mcp.Description("Story description/body"),
		),
		mcp.WithString("story_type",
			mcp.Description("Story type: feature, bug, or chore (default: feature)"),
		),
		mcp.WithNumber("estimate",
			mcp.Description("Point estimate"),
		),
		mcp.WithString("labels",
			mcp.Description("Comma-separated label names"),
		),
	), handleCreateStory)

	s.AddTool(mcp.NewTool("get_project_activity",
		mcp.WithDescription("Get recent activity for a project"),
		mcp.WithTitleAnnotation("Project Activity"),
		mcp.WithNumber("project_id",
			mcp.Description("LiteTracker project ID"),
			mcp.Required(),
		),
		mcp.WithString("occurred_after",
			mcp.Description("Only show activity after this date (e.g. '2026-02-01T00:00:00Z')"),
		),
	), handleGetProjectActivity)

	s.AddTool(mcp.NewTool("add_label",
		mcp.WithDescription("Add a label to a story"),
		mcp.WithTitleAnnotation("Add Label"),
		mcp.WithNumber("project_id",
			mcp.Description("LiteTracker project ID"),
			mcp.Required(),
		),
		mcp.WithNumber("story_id",
			mcp.Description("Story ID"),
			mcp.Required(),
		),
		mcp.WithString("label",
			mcp.Description("Label name to add"),
			mcp.Required(),
		),
	), handleAddLabel)

	s.AddTool(mcp.NewTool("add_owner",
		mcp.WithDescription("Add an owner to a story"),
		mcp.WithTitleAnnotation("Add Owner"),
		mcp.WithNumber("project_id",
			mcp.Description("LiteTracker project ID"),
			mcp.Required(),
		),
		mcp.WithNumber("story_id",
			mcp.Description("Story ID"),
			mcp.Required(),
		),
		mcp.WithNumber("user_id",
			mcp.Description("User ID to add as owner (e.g. 568 for Robert)"),
			mcp.Required(),
		),
	), handleAddOwner)

	return s
}

func textResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: string(b)},
		},
	}, nil
}

func errResult(err error) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
		},
	}, nil
}

func getInt(req mcp.CallToolRequest, key string) int {
	args := req.GetArguments()
	v, ok := args[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

func getString(req mcp.CallToolRequest, key string) string {
	args := req.GetArguments()
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func handleListProjects(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projects, err := api.ListProjects()
	if err != nil {
		return errResult(err)
	}
	type summary struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	out := make([]summary, len(projects))
	for i, p := range projects {
		out[i] = summary{ID: p.ID, Name: p.Title, Description: p.Description}
	}
	return textResult(out)
}

func handleListStories(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectID := getInt(req, "project_id")
	if projectID == 0 {
		return errResult(fmt.Errorf("project_id is required"))
	}
	opts := api.ListStoriesOpts{
		Filter:      getString(req, "filter"),
		Query:       getInt(req, "query"),
		Owners:      getInt(req, "owners"),
		SectionType: getString(req, "section_type"),
		OwnedBy:     getInt(req, "owned_by"),
		State:       getString(req, "state"),
		Limit:       getInt(req, "limit"),
	}
	stories, err := api.ListStories(projectID, opts)
	if err != nil {
		return errResult(err)
	}
	type summary struct {
		ID       int      `json:"id"`
		Name     string   `json:"name"`
		Type     string   `json:"type"`
		State    string   `json:"state"`
		Labels   []string `json:"labels"`
		Estimate *int     `json:"estimate"`
		URL      string   `json:"url"`
	}
	out := make([]summary, len(stories))
	for i, s := range stories {
		labels := make([]string, len(s.Labels))
		for j, l := range s.Labels {
			labels[j] = l.Name
		}
		out[i] = summary{
			ID: s.ID, Name: s.Title, Type: s.StoryType,
			State: s.CurrentState, Labels: labels, Estimate: s.Estimate, URL: s.URL,
		}
	}
	return textResult(out)
}

func handleGetStory(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectID := getInt(req, "project_id")
	storyID := getInt(req, "story_id")
	if projectID == 0 || storyID == 0 {
		return errResult(fmt.Errorf("project_id and story_id are required"))
	}

	story, err := api.GetStory(projectID, storyID)
	if err != nil {
		return errResult(err)
	}
	comments, err := api.GetStoryComments(projectID, storyID)
	if err != nil {
		return errResult(err)
	}

	type commentSummary struct {
		ID        int    `json:"id"`
		Text      string `json:"text"`
		PersonID  int    `json:"person_id"`
		CreatedAt string `json:"created_at"`
	}
	type result struct {
		ID          int              `json:"id"`
		Name        string           `json:"name"`
		Description string           `json:"description"`
		Type        string           `json:"type"`
		State       string           `json:"state"`
		Labels      []string         `json:"labels"`
		Estimate    *int             `json:"estimate"`
		OwnerIDs    []int            `json:"owner_ids"`
		URL         string           `json:"url"`
		CreatedAt   string           `json:"created_at"`
		UpdatedAt   string           `json:"updated_at"`
		Comments    []commentSummary `json:"comments"`
	}

	labels := make([]string, len(story.Labels))
	for i, l := range story.Labels {
		labels[i] = l.Name
	}
	cs := make([]commentSummary, len(comments))
	for i, c := range comments {
		cs[i] = commentSummary{ID: c.ID, Text: c.Text, PersonID: c.PersonID, CreatedAt: c.CreatedAt}
	}

	return textResult(result{
		ID: story.ID, Name: story.Title, Description: story.Description,
		Type: story.StoryType, State: story.CurrentState, Labels: labels,
		Estimate: story.Estimate, OwnerIDs: story.OwnerIDs, URL: story.URL,
		CreatedAt: story.CreatedAt, UpdatedAt: story.UpdatedAt, Comments: cs,
	})
}

func handleGetStoryComments(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectID := getInt(req, "project_id")
	storyID := getInt(req, "story_id")
	if projectID == 0 || storyID == 0 {
		return errResult(fmt.Errorf("project_id and story_id are required"))
	}

	comments, err := api.GetStoryComments(projectID, storyID)
	if err != nil {
		return errResult(err)
	}

	type summary struct {
		ID        int    `json:"id"`
		Text      string `json:"text"`
		PersonID  int    `json:"person_id"`
		CreatedAt string `json:"created_at"`
	}
	out := make([]summary, len(comments))
	for i, c := range comments {
		out[i] = summary{ID: c.ID, Text: c.Text, PersonID: c.PersonID, CreatedAt: c.CreatedAt}
	}
	return textResult(out)
}

func handleCreateStory(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectID := getInt(req, "project_id")
	title := getString(req, "title")
	if projectID == 0 || title == "" {
		return errResult(fmt.Errorf("project_id and title are required"))
	}

	params := map[string]any{"name": title}

	if desc := getString(req, "description"); desc != "" {
		params["description"] = desc
	}
	if st := getString(req, "story_type"); st != "" {
		params["story_type"] = st
	}
	if est := getInt(req, "estimate"); est != 0 {
		params["estimate"] = est
	}
	if labels := getString(req, "labels"); labels != "" {
		labelList := []map[string]string{}
		for _, l := range strings.Split(labels, ",") {
			l = strings.TrimSpace(l)
			if l != "" {
				labelList = append(labelList, map[string]string{"name": l})
			}
		}
		params["labels"] = labelList
	}

	story, err := api.CreateStory(projectID, params)
	if err != nil {
		return errResult(err)
	}

	type result struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Type  string `json:"type"`
		State string `json:"state"`
		URL   string `json:"url"`
	}
	return textResult(result{
		ID: story.ID, Name: story.Title, Type: story.StoryType,
		State: story.CurrentState, URL: story.URL,
	})
}

func handlePostComment(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectID := getInt(req, "project_id")
	storyID := getInt(req, "story_id")
	text := getString(req, "text")
	if projectID == 0 || storyID == 0 || text == "" {
		return errResult(fmt.Errorf("project_id, story_id, and text are required"))
	}

	comment, err := api.WebPostComment(projectID, storyID, text)
	if err != nil {
		return errResult(err)
	}

	type result struct {
		ID        int    `json:"id"`
		Text      string `json:"text"`
		CreatedAt string `json:"created_at"`
	}
	return textResult(result{ID: comment.ID, Text: comment.Text, CreatedAt: comment.CreatedAt})
}

func handleGetMe(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	me, err := api.GetMe()
	if err != nil {
		return errResult(err)
	}
	type project struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Role string `json:"role"`
	}
	type result struct {
		ID       int       `json:"id"`
		Name     string    `json:"name"`
		Username string    `json:"username"`
		Email    string    `json:"email"`
		Initials string    `json:"initials"`
		Projects []project `json:"projects"`
	}
	projects := make([]project, len(me.Projects))
	for i, p := range me.Projects {
		projects[i] = project{ID: p.ProjectID, Name: p.ProjectName, Role: p.Role}
	}
	return textResult(result{
		ID: me.ID, Name: me.Name, Username: me.Username,
		Email: me.Email, Initials: me.Initials, Projects: projects,
	})
}

func handleGetProjectActivity(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectID := getInt(req, "project_id")
	if projectID == 0 {
		return errResult(fmt.Errorf("project_id is required"))
	}
	occurredAfter := getString(req, "occurred_after")
	if occurredAfter == "" {
		occurredAfter = time.Now().AddDate(0, 0, -7).Format(time.RFC3339)
	}

	activities, err := api.GetProjectActivity(projectID, occurredAfter)
	if err != nil {
		return errResult(err)
	}

	type resource struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	type summary struct {
		Message    string     `json:"message"`
		PerformedBy string   `json:"performed_by"`
		OccurredAt string    `json:"occurred_at"`
		Resources  []resource `json:"resources"`
	}
	out := make([]summary, len(activities))
	for i, a := range activities {
		resources := make([]resource, len(a.PrimaryResources))
		for j, r := range a.PrimaryResources {
			resources[j] = resource{Name: r.Name, URL: r.URL}
		}
		out[i] = summary{
			Message:     a.Message,
			PerformedBy: a.PerformedBy.Name,
			OccurredAt:  a.OccurredAt,
			Resources:   resources,
		}
	}
	return textResult(out)
}

func handleAddLabel(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectID := getInt(req, "project_id")
	storyID := getInt(req, "story_id")
	label := getString(req, "label")
	if projectID == 0 || storyID == 0 || label == "" {
		return errResult(fmt.Errorf("project_id, story_id, and label are required"))
	}

	result, err := api.WebAddLabel(projectID, storyID, label)
	if err != nil {
		return errResult(err)
	}

	type labelResult struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	return textResult(labelResult{ID: result.ID, Name: result.Name})
}

func handleAddOwner(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectID := getInt(req, "project_id")
	storyID := getInt(req, "story_id")
	userID := getInt(req, "user_id")
	if projectID == 0 || storyID == 0 || userID == 0 {
		return errResult(fmt.Errorf("project_id, story_id, and user_id are required"))
	}

	owners, err := api.WebAddOwner(projectID, storyID, userID)
	if err != nil {
		return errResult(err)
	}

	type ownerSummary struct {
		UserID   int    `json:"user_id"`
		Name     string `json:"name"`
		Initials string `json:"initials"`
	}
	out := make([]ownerSummary, len(owners))
	for i, o := range owners {
		out[i] = ownerSummary{UserID: o.UserID, Name: o.Name, Initials: o.Initials}
	}
	return textResult(out)
}
