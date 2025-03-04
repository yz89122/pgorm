/*
internal is a private internal package.
*/
package internal

import (
	"math/rand"
	"time"
)

func RetryBackoff(retry int, minBackoff, maxBackoff time.Duration) time.Duration {
	if retry < 0 {
		panic("not reached")
	}
	if minBackoff == 0 {
		return 0
	}

	d := minBackoff << uint(retry)
	d = minBackoff + time.Duration(rand.Int63n(int64(d))) //nolint:gosec

	if d > maxBackoff || d < minBackoff {
		d = maxBackoff
	}

	return d
}
