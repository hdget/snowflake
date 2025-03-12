package snowflake

import (
	"time"
)

// ID snowflake id
type ID struct {
	startTime time.Time
	Sequence  uint64
	Node      uint64
	Timestamp uint64
}

func (i ID) GetTime() time.Time {
	ms := i.startTime.UTC().UnixNano()/1e6 + int64(i.Timestamp)
	return time.Unix(0, ms*int64(time.Millisecond)).UTC()
}
