package snowflake

import (
	"errors"
	"time"
)

type Option func(algorithm *Algorithm) error

// WithStartTime set the start time for snowflake algorithm.
//
// It will panic when:
//
//	s IsZero
//	s > current millisecond,
//	current millisecond - s > 2^41(69 years).
//
// This function is thread-unsafe, recommended you call him in the main function.
func WithStartTime(t time.Time) Option {
	return func(a *Algorithm) error {
		t = t.UTC()

		if t.IsZero() {
			return errors.New("The start time cannot be a zero value")
		}

		if t.After(time.Now().UTC()) {
			return errors.New("The t cannot be greater than the current millisecond")
		}

		// since we check the current millisecond is greater than t, so we don't need to check the overflow.
		df := elapsedTime(currentMillis(), t)
		if uint64(df) > maxTimestamp {
			return errors.New("The maximum life cycle of the snowflake algorithm is 69 years")
		}
		a.startTime = t
		return nil
	}
}

func WithNodeBits(nodeBits uint8) Option {
	return func(a *Algorithm) error {
		if nodeBits == 0 {
			return errors.New("invalid node bits")
		}

		// 有可能多个服务运行snowflake服务，但defaultNodeBits有限, nodeNumber不能大于nodeMax
		if nodeBits > 10 {
			return errors.New("the node bits cannot be greater than 10")
		}

		a.nodeBits = nodeBits
		return nil
	}
}

func WithSequenceBits(sequenceBits uint8) Option {
	return func(a *Algorithm) error {
		if sequenceBits == 0 {
			return errors.New("invalid sequence bits")
		}

		// 有可能多个服务运行snowflake服务，但defaultNodeBits有限, nodeNumber不能大于nodeMax
		if sequenceBits > 12 {
			return errors.New("the nodeId cannot be greater than 12")
		}

		a.sequenceBits = sequenceBits
		return nil
	}
}
