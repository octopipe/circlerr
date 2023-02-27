package domain

const (
	DefaultWorkspaceType = "DEFAULT"
	MatchWorkspaceType   = "MATCH"
	CanaryWorkspaceType  = "CANARY"
)

type Workspace struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
	Type        string `json:"type" default:"DEFAULT"`
}
