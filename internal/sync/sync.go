package sync

import (
	"log/slog"
	"strings"

	"github.com/MelianLabs/litetracker-mcp/internal/api"
	"github.com/MelianLabs/litetracker-mcp/internal/config"
	"github.com/MelianLabs/litetracker-mcp/internal/db"
)

func fetchAllStories(projectID int, state string) []api.Story {
	stories, err := api.ListStories(projectID, api.ListStoriesOpts{State: state, Limit: 200})
	if err != nil {
		slog.Error("failed to fetch stories", "projectID", projectID, "state", state, "err", err)
		return nil
	}
	return stories
}

func isMyStory(story api.Story) bool {
	for _, o := range story.Owners {
		if o.UserID == config.C.UserID {
			return true
		}
	}
	return false
}

func mentionsUser(text string) bool {
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	username := strings.ToLower(config.C.Username)
	return strings.Contains(lower, username) || strings.Contains(lower, "@"+username)
}

type syncStats struct {
	Stories  int
	Mine     int
	Comments int
}

func syncProject(projectID int) syncStats {
	stats := syncStats{}

	states := []string{"started", "unstarted", "delivered", "accepted", "rejected"}
	var allStories []api.Story
	for _, state := range states {
		allStories = append(allStories, fetchAllStories(projectID, state)...)
	}

	myStoryIDs := map[int]bool{}
	for _, s := range allStories {
		if isMyStory(s) {
			myStoryIDs[s.ID] = true
		}
	}

	// Upsert all stories
	for _, s := range allStories {
		isMine := myStoryIDs[s.ID]
		ownerNames := make([]string, len(s.Owners))
		for i, o := range s.Owners {
			ownerNames[i] = o.Name
		}
		labelNames := make([]string, len(s.Labels))
		for i, l := range s.Labels {
			labelNames[i] = l.Name
		}

		row := db.StoryRow{
			ID:           s.ID,
			ProjectID:    projectID,
			Title:        s.Title,
			IsMine:       isMine,
			MentionsMe:   false,
			CreatedAt:    s.CreatedAt,
			UpdatedAt:    s.UpdatedAt,
		}
		if s.Description != "" {
			row.Description = &s.Description
		}
		if s.StoryType != "" {
			row.StoryType = &s.StoryType
		}
		if s.CurrentState != "" {
			row.CurrentState = &s.CurrentState
		}
		row.Estimate = s.Estimate
		if s.StoryPriority != "" {
			row.Priority = &s.StoryPriority
		}
		if s.URL != "" {
			row.URL = &s.URL
		}
		row.RequestedByID = s.RequestedByID
		if len(ownerNames) > 0 {
			joined := strings.Join(ownerNames, ", ")
			row.OwnerNames = &joined
		}
		if len(labelNames) > 0 {
			joined := strings.Join(labelNames, ", ")
			row.LabelNames = &joined
		}

		if err := db.UpsertStory(row); err != nil {
			slog.Error("upsert story failed", "storyID", s.ID, "err", err)
			continue
		}
		stats.Stories++
	}

	stats.Mine = len(myStoryIDs)

	// Fetch and sync comments for all stories
	for _, s := range allStories {
		comments, err := api.GetStoryComments(projectID, s.ID)
		if err != nil {
			slog.Error("failed to fetch comments", "storyID", s.ID, "err", err)
			continue
		}
		for _, c := range comments {
			mentions := mentionsUser(c.Text)
			row := db.CommentRow{
				ID:         c.ID,
				StoryID:    s.ID,
				ProjectID:  projectID,
				MentionsMe: mentions,
				CreatedAt:  c.CreatedAt,
			}
			if c.Text != "" {
				row.Text = &c.Text
			}
			if c.PersonID != 0 {
				row.PersonID = &c.PersonID
			}
			if c.Person != nil && c.Person.Name != "" {
				row.PersonName = &c.Person.Name
			}

			if err := db.UpsertComment(row); err != nil {
				slog.Error("upsert comment failed", "commentID", c.ID, "err", err)
				continue
			}
			stats.Comments++
			if mentions {
				_ = db.MarkStoryMentionsMe(s.ID)
			}
		}
	}

	return stats
}

func SyncAllProjects() {
	slog.Info("starting story sync")

	for _, pid := range config.C.ProjectIDs {
		stats := syncProject(pid)
		slog.Info("synced project",
			"projectID", pid,
			"stories", stats.Stories,
			"mine", stats.Mine,
			"comments", stats.Comments,
		)
	}

	if err := db.CreateSnapshot(); err != nil {
		slog.Error("snapshot creation failed", "err", err)
	} else {
		slog.Info("snapshot created")
	}

	slog.Info("story sync complete")
}
