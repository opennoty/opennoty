package health

import (
	"errors"
	"sync"
	"sync/atomic"
)

var (
	ErrAlreadyRegistered = errors.New("already registered")
)

type Provider interface {
	Check() Health
}

type providerHolder struct {
	name     string
	provider Provider
	critical bool
	enabled  bool
}

type Checker struct {
	providers map[string]*providerHolder
}

func NewChecker() *Checker {
	return &Checker{
		providers: map[string]*providerHolder{},
	}
}

func (h *Checker) Register(name string, critical bool, provider Provider) error {
	_, ok := h.providers[name]
	if ok {
		return ErrAlreadyRegistered
	}
	h.providers[name] = &providerHolder{
		name:     name,
		provider: provider,
		critical: critical,
		enabled:  true,
	}
	return nil
}

func (h *Checker) SetEnabled(providerName string, enabled bool) {
	holder, ok := h.providers[providerName]
	if ok {
		holder.enabled = enabled
	}
}

func (h *Checker) Check() *Response {
	var wg sync.WaitGroup
	var totalResult int32 = 1

	response := &Response{
		Details: map[string]Health{},
	}

	for _, holder := range h.providers {
		if holder.enabled {
			wg.Add(1)
			go func(holder *providerHolder) {
				result := holder.provider.Check()
				response.Details[holder.name] = result
				if result.Status != StatusUp && holder.critical {
					atomic.SwapInt32(&totalResult, 0)
				}
				wg.Done()
			}(holder)
		} else {
			response.Details[holder.name] = Health{
				Status: StatusDisabled,
			}
		}
	}
	wg.Wait()

	if totalResult == 1 {
		response.Status = StatusUp
	} else {
		response.Status = StatusDown
	}

	return response
}
