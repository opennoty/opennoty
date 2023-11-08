package server

type Authentication interface {
	GetTenantId() string
	GetAccountId() string
	CheckSubscribeTopic(topic string) bool
}

type ClientContext struct {
	authentication Authentication
}

const clientContextKey = "opennoty.ClientContext"

func (c *ClientContext) GetAuthentication() Authentication {
	return c.authentication
}

func (c *ClientContext) SetAuthentication(authentication Authentication) {
	c.authentication = authentication
}

func (c *ClientContext) GetTenantId() string {
	if c.authentication == nil {
		return ""
	}
	return c.authentication.GetTenantId()
}

func (c *ClientContext) GetAccountId() string {
	if c.authentication == nil {
		return ""
	}
	return c.authentication.GetAccountId()
}

func (c *ClientContext) CheckSubscribeTopic(topic string) bool {
	if c.authentication == nil {
		return true
	}
	return c.authentication.CheckSubscribeTopic(topic)
}
