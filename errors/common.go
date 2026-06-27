package errors

func InvalidParamsErr(err error) error {
	return E(Invalid, "invalid params", err)
}

func InvalidBodyErr(err error) error {
	return E(Invalid, "invalid request body", err)
}

func NotFoundErr(err error) error {
	return E(NotFound, err)
}

func ForbiddenErr(err error) error {
	return E(Forbidden, err)
}

func UnauthorizedErr(err error) error {
	return E(Unauthorized, err)
}

func ConflictErr(err error) error {
	return E(Conflict, err)
}

func InvalidErr(err error) error {
	return E(Invalid, err)
}
