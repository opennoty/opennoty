package workflow_service

import "regexp"

var regexpWorkflowName = regexp.MustCompile("^[a-zA-Z0-9._-]*$")

func CheckWorkflowName(name string) bool {
	return len(name) > 0 && regexpWorkflowName.MatchString(name)
}
