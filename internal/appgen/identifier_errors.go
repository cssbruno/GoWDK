package appgen

type generatedIdentifierError struct {
	message string
}

func (err generatedIdentifierError) Error() string {
	return err.message
}

func recoverGeneratedIdentifierError(err *error) {
	recovered := recover()
	if recovered == nil {
		return
	}
	if generatedErr, ok := recovered.(generatedIdentifierError); ok {
		*err = generatedErr
		return
	}
	panic(recovered)
}
