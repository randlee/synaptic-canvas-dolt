package operations

type OperationError struct {
	Operation string
	Err       error
}

func (e OperationError) Error() string {
	if e.Err == nil {
		return e.Operation
	}
	return e.Err.Error()
}

func (e OperationError) Unwrap() error {
	return e.Err
}

func WrapOperation(operation string, err error) error {
	if err == nil {
		return nil
	}
	return OperationError{Operation: operation, Err: err}
}

func OperationName(err error) string {
	var target OperationError
	if !AsOperationError(err, &target) {
		return ""
	}
	return target.Operation
}

func AsOperationError(err error, target *OperationError) bool {
	if err == nil || target == nil {
		return false
	}
	current := err
	for current != nil {
		if opErr, ok := current.(OperationError); ok {
			*target = opErr
			return true
		}
		type unwrapper interface{ Unwrap() error }
		next, ok := current.(unwrapper)
		if !ok {
			return false
		}
		current = next.Unwrap()
	}
	return false
}
