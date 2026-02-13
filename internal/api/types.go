package api

type Project struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Public      bool   `json:"public"`
}

type Label struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type Person struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Initials string `json:"initials"`
	Kind     string `json:"kind"`
}

type StoryOwner struct {
	ID       int    `json:"id"`
	UserID   int    `json:"user_id"`
	Name     string `json:"name"`
	Initials string `json:"initials"`
}

type Story struct {
	ID            int          `json:"id"`
	Title         string       `json:"title"`
	Description   string       `json:"description,omitempty"`
	StoryType     string       `json:"story_type"`
	CurrentState  string       `json:"current_state"`
	Estimate      *int         `json:"estimate,omitempty"`
	StoryPriority string       `json:"story_priority,omitempty"`
	Labels        []Label      `json:"labels"`
	OwnerIDs      []int        `json:"owner_ids"`
	Owners        []StoryOwner `json:"owners,omitempty"`
	RequestedByID *int         `json:"requested_by_id,omitempty"`
	CreatedAt     string       `json:"created_at"`
	UpdatedAt     string       `json:"updated_at"`
	URL           string       `json:"url"`
	ProjectID     *int         `json:"project_id,omitempty"`
}

type Comment struct {
	ID        int     `json:"id"`
	Text      string  `json:"text"`
	PersonID  int     `json:"person_id"`
	Person    *Person `json:"person,omitempty"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type ActivityChange struct {
	Kind       string         `json:"kind"`
	ID         int            `json:"id"`
	ChangeType string         `json:"change_type"`
	NewValues  map[string]any `json:"new_values,omitempty"`
}

type ActivityResource struct {
	Kind      string `json:"kind"`
	ID        int    `json:"id"`
	Name      string `json:"name"`
	StoryType string `json:"story_type,omitempty"`
	URL       string `json:"url"`
}

type Activity struct {
	Kind             string             `json:"kind"`
	GUID             string             `json:"guid"`
	Message          string             `json:"message"`
	PerformedBy      Person             `json:"performed_by"`
	OccurredAt       string             `json:"occurred_at"`
	Changes          []ActivityChange   `json:"changes"`
	PrimaryResources []ActivityResource `json:"primary_resources"`
}

type AccountSummary struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ProjectMembership struct {
	ProjectID   int    `json:"project_id"`
	ProjectName string `json:"project_name"`
	Role        string `json:"role"`
}

type Me struct {
	ID       int                 `json:"id"`
	Name     string              `json:"name"`
	Initials string              `json:"initials"`
	Username string              `json:"username"`
	Email    string              `json:"email"`
	Accounts []AccountSummary    `json:"accounts"`
	Projects []ProjectMembership `json:"projects"`
}

type ListStoriesOpts struct {
	Filter      string
	Query       int
	Owners      int
	SectionType string
	OwnedBy     int
	State       string
	Limit       int
}
