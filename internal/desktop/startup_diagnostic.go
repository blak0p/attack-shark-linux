package desktop

// Keep diagnostic errors classified: underlying errors may contain device paths.
func errorCodeOrEmpty(err error) ErrorCode {
	if err == nil {
		return ""
	}
	return errorCode(err, true)
}
