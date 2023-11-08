package api_model

type Notification struct {
	Id         string `json:"id"`
	Subject    string `json:"subject"`
	Step       Step   `json:"step"`
	ReadMarked bool   `json:"readMarked"`
	Deleted    bool   `json:"deleted"`
}
