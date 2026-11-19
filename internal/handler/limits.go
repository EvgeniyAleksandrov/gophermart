package handler

import "time"

const (
	maxCredentialsBodyBytes = 64 * 1024
	httpOperationTimeout    = 25 * time.Second
)
