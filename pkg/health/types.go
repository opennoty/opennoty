package health

type Status = string

const (
	StatusUp       Status = "UP"
	StatusDown     Status = "DOWN"
	StatusDisabled Status = "DISABLED"
)

type Health struct {
	Status Status `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type Response struct {
	Status  Status            `json:"status"`
	Details map[string]Health `json:"details"`
}
