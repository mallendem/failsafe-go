package test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/fallback"
	"github.com/failsafe-go/failsafe-go/internal/policytesting"
	"github.com/failsafe-go/failsafe-go/internal/testutil"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

/*
RetryPolicy -> RetryPolicy

Tests a scenario with nested retry policies where the inner policy is exceeded and skipped.
*/
func TestNestedRetryPoliciesWhereInnerIsExceeded(t *testing.T) {
	// Given
	outerRetryStats := &policytesting.Stats{}
	innerRetryStats := &policytesting.Stats{}
	outerRetryPolicy := policytesting.WithRetryStatsAndLogs(retrypolicy.Builder[bool]().WithMaxRetries(10), outerRetryStats).Build()
	innerRetryPolicy := policytesting.WithRetryStatsAndLogs(retrypolicy.Builder[bool]().WithMaxRetries(1), innerRetryStats).Build()
	fn, reset := testutil.ErrorNTimesThenReturn[bool](testutil.ErrConnecting, 5, true)
	setup := func() {
		reset()
		outerRetryStats.Reset()
		innerRetryStats.Reset()
	}

	// When / Then
	testutil.Test[bool](t).
		With(outerRetryPolicy, innerRetryPolicy).
		Setup(setup).
		Get(fn).
		AssertSuccess(6, 6, true, func() {
			assert.Equal(t, 5, outerRetryStats.Executions())
			assert.Equal(t, 4, outerRetryStats.Failures())
			assert.Equal(t, 1, outerRetryStats.Successes())
			assert.Equal(t, 2, innerRetryStats.Executions())
			assert.Equal(t, 2, innerRetryStats.Failures())
			assert.Equal(t, 0, innerRetryStats.Successes())
		})
}

/*
RetryPolicy -> RetryPolicy

Asserts that attempt counts are as expected when using nested retry policies.
*/
func TestRetryPolicyRetryPolicyAttempts(t *testing.T) {
	rp1 := policytesting.WithRetryLogs(retrypolicy.Builder[any]()).Build()
	rp2 := policytesting.WithRetryLogs(retrypolicy.Builder[any]()).Build()
	testutil.Test[any](t).
		With(rp2, rp1).
		Get(testutil.GetFn[any](nil, testutil.ErrInvalidArgument)).
		AssertFailure(5, 5, testutil.ErrInvalidArgument)
}

/*
Fallback -> RetryPolicy -> RetryPolicy
*/
func TestFallbackRetryPolicyRetryPolicy(t *testing.T) {
	// Given
	retryPolicy1Stats := &policytesting.Stats{}
	retryPolicy2Stats := &policytesting.Stats{}
	retryPolicy1 := policytesting.WithRetryStats(retrypolicy.Builder[any]().HandleErrors(testutil.ErrInvalidState).WithMaxRetries(2), retryPolicy1Stats).Build()
	retryPolicy2 := policytesting.WithRetryStats(retrypolicy.Builder[any]().HandleErrors(testutil.ErrInvalidArgument).WithMaxRetries(3), retryPolicy2Stats).Build()
	fb := fallback.WithResult[any](true)
	fn := func(exec failsafe.Execution[any]) (any, error) {
		if exec.Attempts()%2 == 1 {
			return false, testutil.ErrInvalidState
		}
		return false, testutil.ErrInvalidArgument
	}
	setup := func() {
		retryPolicy1Stats.Reset()
		retryPolicy2Stats.Reset()
	}

	// When / Then
	testutil.Test[any](t).
		With(fb, retryPolicy2, retryPolicy1).
		Setup(setup).
		Get(fn).
		AssertSuccess(5, 5, true, func() {
			// Expected RetryPolicy failure sequence:
			//    rp1 ErrInvalidState - failure, retry
			//    rp1 ErrInvalidArgument - success
			//    rp2 ErrInvalidArgument - failure, retry
			//    rp1 ErrInvalidState - failure, retry, retries exhausted
			//    rp1 ErrInvalidArgument - success
			//    rp2 ErrInvalidArgument - failure, retry
			//    rp1 ErrInvalidState - failure, retries exceeded
			//    rp2 ErrInvalidState - success
			assert.Equal(t, 5, retryPolicy1Stats.Executions())
			assert.Equal(t, 3, retryPolicy1Stats.Failures())
			assert.Equal(t, 2, retryPolicy1Stats.Successes())
			assert.Equal(t, 3, retryPolicy2Stats.Executions())
			assert.Equal(t, 2, retryPolicy2Stats.Failures())
			assert.Equal(t, 1, retryPolicy2Stats.Successes())
		})

	// When / Then
	testutil.Test[any](t).
		With(fb, retryPolicy1, retryPolicy2).
		Setup(setup).
		Get(fn).
		AssertSuccess(5, 5, true, func() {
			// Expected RetryPolicy failure sequence:
			//    rp2 ErrInvalidState - success
			//    rp1 ErrInvalidState - failure, retry
			//    rp2 ErrInvalidArgument - failure, retry
			//    rp2 ErrInvalidState - success
			//    rp1 ErrInvalidState - failure, retry, retries exhausted
			//    rp2 ErrInvalidArgument - failure, retry
			//    rp2 ErrInvalidState - success
			//    rp1 ErrInvalidState - retries exceeded
			assert.Equal(t, 3, retryPolicy1Stats.Executions())
			assert.Equal(t, 3, retryPolicy1Stats.Failures())
			assert.Equal(t, 0, retryPolicy1Stats.Successes())
			assert.Equal(t, 5, retryPolicy2Stats.Executions())
			assert.Equal(t, 2, retryPolicy2Stats.Failures())
			assert.Equal(t, 3, retryPolicy2Stats.Successes())
		})
}
