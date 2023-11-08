package mailsender

import "net/textproto"

type SendMailError struct {
	errorMessage string
	Cause        error
	Code         int
	Msg          string
}

func (s *SendMailError) Error() string {
	return s.errorMessage
}

func wrapToSendMailError(err error) *SendMailError {
	textprotoErr, ok := err.(*textproto.Error)
	if ok {
		return &SendMailError{
			errorMessage: err.Error(),
			Cause:        err,
			Code:         textprotoErr.Code,
			Msg:          textprotoErr.Msg,
		}
	}
	return &SendMailError{
		errorMessage: err.Error(),
		Cause:        err,
	}
}
