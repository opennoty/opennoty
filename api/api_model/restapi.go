package api_model

type Workflow struct {
	Name     string              `json:"name"`
	Revision int64               `json:"revision"`
	Flow     []*WorkflowFlowItem `json:"flow"`
}

type WorkflowTriggerRequest struct {
	Tenant     Tenant         `json:"tenant"`
	Subscriber Subscriber     `json:"subscriber"`
	Event      map[string]any `json:"event"`
}

type WorkflowTriggerResponse struct {
	EventId string `json:"eventId"`
}
