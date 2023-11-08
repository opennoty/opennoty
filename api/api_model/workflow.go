package api_model

import (
	"encoding/json"
)

type Subscriber struct {
	AccountId string `json:"accountId" bson:"accountId"`
	FullName  string `json:"fullName" bson:"fullName"`
	Email     string `json:"email" bson:"email"`
	Locale    string `json:"locale" bson:"locale"`
}

type Tenant struct {
	Id   string         `json:"id" bson:"id"`
	Name string         `json:"name" bson:"name"`
	Data map[string]any `json:"data" bson:"data"`
}

type Step struct {
	Digest     bool              `json:"digest"`
	Events     []json.RawMessage `json:"events"`
	TotalCount int               `json:"totalCount"`
}

type FlowType string

const (
	FlowNotification FlowType = "notification"
	FlowEmail        FlowType = "email"
	FlowDigest       FlowType = "digest"
)

type DigestTiming string

const (
	DigestEvent    DigestTiming = "event"
	DigestSchedule DigestTiming = "schedule"
)

type DigestFlow struct {
	Timing DigestTiming `bson:"timing" json:"timing"`

	// EventTime Timing == DigestEvent, seconds
	EventTime int `bson:"eventTime" json:"eventTime"`

	//// EventTime Timing == DigestSchedule, cron expression
	//ScheduleCron string `bson:"scheduleTime" json:"scheduleTime"`
}

type WorkflowFlowItem struct {
	Type FlowType `bson:"type" json:"type"`

	// Type == FlowDigest
	Digest *DigestFlow `bson:"digest,omitempty" json:"digest,omitempty"`

	// Type == FlowNotification or FlowEmail
	SubjectTemplate string `bson:"subjectTemplate,omitempty" json:"subjectTemplate,omitempty"`

	// Type == FlowEmail
	ContentTemplate string `bson:"contentTemplate,omitempty" json:"contentTemplate,omitempty"`
}
