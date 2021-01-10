package cc

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
	_, ok := err.(*CompilerError)
	return ok
}
