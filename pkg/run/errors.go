package run

import "errors"

type CompilerError struct {
	error
	text string
}

func NewCompilerError(text string) *CompilerError {
	return &CompilerError{
		text: text,
	}
}

func (e *CompilerError) Error() string {
	return e.text
}

func IsCompilerError(err error) bool {
	return errors.As(err, &CompilerError{})
}
