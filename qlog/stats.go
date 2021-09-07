package qlog

import "time"

type bufferStats struct {
	PlayoutTime   time.Duration
	PlayoutBytes  int64
	PlayoutFrames int32
	MaxTime       time.Duration
	MaxBytes      int64
	MaxFrames     int32
}

func NewBufferStats() bufferStats {
	return bufferStats{
		PlayoutTime:   0,
		PlayoutBytes:  -1,
		PlayoutFrames: -1,
		MaxTime:       0,
		MaxBytes:      -1,
		MaxFrames:     -1,
	}
}
