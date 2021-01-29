package clientpool_test

import (
	"github.com/reddit/baseplate.go/clientpool"
	"github.com/reddit/baseplate.go/retrybp"
)

// Doing this check in test code to avoid making clientpool importing retrybp.
var _ retrybp.RetryableError = clientpool.ErrExhausted
