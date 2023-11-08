package workflow_service

type eventTriggerEventMessage struct {
	TenantId string `json:"tenantId"`
	EventId  string `json:"eventId"`
}
