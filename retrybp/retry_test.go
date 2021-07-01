package retrybp_test

import (
	"context"
	"errors"
	"testing"
	"time"

	retry "github.com/avast/retry-go"

	"github.com/reddit/baseplate.go/errorsbp"
	"github.com/reddit/baseplate.go/retrybp"
)

func TestDefaultOverwrites(t *testing.T) {
	t.Parallel()

	if retry.DefaultAttempts != 1 {
		t.Errorf("retry.DefaultAttempts was not overwritten: %v", retry.DefaultAttempts)
	}
	if retry.DefaultDelay != 1*time.Millisecond {
		t.Errorf("retry.DefaultDelay was not overwritten: %v", retry.DefaultDelay)
	}
	if retry.DefaultMaxJitter != 5*time.Millisecond {
		t.Errorf("retry.DefaultMaxJitter was not overwritten: %v", retry.DefaultMaxJitter)
	}
}

func TestContextOptions(t *testing.T) {
	t.Parallel()

	t.Run(
		"no-options",
		func(t *testing.T) {
			options, ok := retrybp.GetOptions(context.Background())
			if ok {
				t.Errorf("expected ok to be false, got true")
			}
			if options != nil {
				t.Errorf("expected options to be nil, got %v", options)
			}
		},
	)

	t.Run(
		"set-options",
		func(t *testing.T) {
			ctx := retrybp.WithOptions(context.Background(), retry.Attempts(1))

			options, ok := retrybp.GetOptions(ctx)
			if !ok {
				t.Errorf("expected ok to be true, got false")
			}
			if len(options) != 1 {
				t.Errorf("unexpected number of options, got %v", options)
			}
		},
	)

	t.Run(
		"set-no-options",
		func(t *testing.T) {
			ctx := retrybp.WithOptions(context.Background())

			options, ok := retrybp.GetOptions(ctx)
			if !ok {
				t.Errorf("expected ok to be true, got false")
			}
			if len(options) != 0 {
				t.Errorf("unexpected number of options, got %v", options)
			}
		},
	)
}

func TestDoBatchError(t *testing.T) {
	t.Parallel()

	errList := []error{
		errors.New("foo"),
		errors.New("bar"),
		errors.New("fizz"),
		errors.New("buzz"),
	}
	i := 0

	err := retrybp.Do(
		context.TODO(),
		func() error {
			idx := i
			i++
			return errList[idx]
		},
		retry.Attempts(uint(len(errList))),
	)

	var batchErr errorsbp.Batch
	if errors.As(err, &batchErr) {
		if len(batchErr.GetErrors()) != len(errList) {
			t.Errorf(
				"wrong number of errors in %v, expected %v",
				batchErr.GetErrors(),
				errList,
			)

			for _, err := range errList {
				if !errors.Is(batchErr, err) {
					t.Errorf("%v is not wrapped by %v", err, batchErr)
				}
			}
		}
	} else {
		t.Fatalf("unexpected error type %v", err)
	}
}

func TestDoContextOverridesDefaults(t *testing.T) {
	t.Parallel()

	ctx := retrybp.WithOptions(context.Background(), retry.Attempts(maxAttempts))
	counter := &counter{}
	retrybp.Do(
		ctx,
		counter.call,
		retry.Attempts(maxAttempts*2),
		retrybp.Filters(doFilter),
	)
	if counter.calls != maxAttempts {
		t.Errorf("number of calls did not match, expected %v, got %v", maxAttempts, counter.calls)
	}
}

func TestSingleAttempt(t *testing.T) {
	t.Parallel()

	start := time.Now()
	delay := time.Minute
	attempts := 0
	_ = retry.Do(
		func() error {
			attempts++
			return errors.New("test")
		},
		retry.Attempts(1),
		retry.Delay(delay),
	)
	duration := time.Since(start)

	// We expect that if we only make a single attempt, then the delay logic will
	// not trigger.
	if attempts != 1 {
		t.Errorf("wrong number of attempts, expected 1, got %d", attempts)
	}
	if duration >= delay {
		t.Errorf("did not expect to trigger delay")
	}
}
