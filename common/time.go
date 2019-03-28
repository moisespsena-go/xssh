package common

import (
	"fmt"
	"time"
)

func TimeToString(t time.Time) (s string) {
	return fmt.Sprintf(
		"%04d%02d%02d%02d%02d%02d%09d",
		t.Year(),
		t.Month(),
		t.Day(),
		t.Hour(),
		t.Minute(),
		t.Second(),
		t.Nanosecond(),
	)
}
