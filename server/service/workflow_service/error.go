package workflow_service

type ErrNotExistsWorkflow struct {
	Name string
}

func (e *ErrNotExistsWorkflow) Error() string {
	return "not exists workflow name: " + e.Name
}
