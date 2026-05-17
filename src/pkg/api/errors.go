package api

type ErrorOptions struct {
	Retryable       bool
	Details         map[string]any
	SuggestedAction string
}

func NewError(code ErrorCode, message string, opts ...ErrorOptions) Error {
	err := Error{
		Code:      code,
		Message:   message,
		Retryable: false,
	}
	if len(opts) == 0 {
		return err
	}
	opt := opts[0]
	err.Retryable = opt.Retryable
	if len(opt.Details) > 0 {
		err.Details = cloneDetails(opt.Details)
	}
	err.SuggestedAction = opt.SuggestedAction
	return err
}

func NewInstallScopeFailure(pkg, scope string, err Error) InstallScopeFailure {
	return InstallScopeFailure{
		Package:         pkg,
		Scope:           scope,
		Code:            err.Code,
		Error:           err.Message,
		Retryable:       err.Retryable,
		Details:         cloneDetails(err.Details),
		SuggestedAction: err.SuggestedAction,
	}
}

func cloneDetails(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}
