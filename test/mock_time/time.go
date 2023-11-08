package mock_time

import "time"

type MockTimePackage struct {
	NowValue time.Time
}

func (m *MockTimePackage) Now() time.Time {
	return m.NowValue
}

func (m *MockTimePackage) SetNow(now time.Time) {
	m.NowValue = now
}

func NewMockTime() *MockTimePackage {
	return &MockTimePackage{
		NowValue: time.Now(),
	}
}
