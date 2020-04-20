package datatypes

import (
	"strconv"
	"time"
)

// TimeM (Time convertable from JSON int representing minisecond)
// encapsulate unix time, converts to and from JSON to Unix time in millisecond
type TimeM struct {
	Time time.Time
}

// UnmarshalJSON unmarshalls it from a string of millisecond
func (t *TimeM) UnmarshalJSON(b []byte) (err error) {
	timestamp := string(b)
	epoch, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return err
	}
	// s := epoch / 1000
	// milliSecLeft := epoch % 1000
	// nanoSec := milliSecLeft * 1e6
	// t.time = time.Unix(s, nanoSec)
	t.Time = time.Unix(0, epoch)

	return err
}

// MarshalJSON customizes unmarshalling from JSON array
func (t *TimeM) MarshalJSON() ([]byte, error) {
	nano := t.Time.UnixNano()
	milli := nano / 1e3
	return []byte(strconv.FormatInt(milli, 10)), nil
}
