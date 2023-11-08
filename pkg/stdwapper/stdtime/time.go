package stdtime

import "time"

var sysTimeInstance sysTimePackage

type TimePackage interface {
	Now() time.Time
}

type sysTimePackage struct {
}

func (s *sysTimePackage) Now() time.Time {
	return time.Now()
}

func SysTime() TimePackage {
	return &sysTimeInstance
}
