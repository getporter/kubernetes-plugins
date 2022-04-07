package secrets

type InvalidSecretDataKeyError struct {
	msg string
}

func (e InvalidSecretDataKeyError) Error() string {
	return e.msg
}
