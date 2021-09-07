package qlog

import (
	"time"

	"github.com/francoispqt/gojay"
)

func milliseconds(dur time.Duration) float64 { return float64(dur.Nanoseconds()) / 1e6 }

type eventDetails interface {
	Category() category
	Name() string
	gojay.MarshalerJSONObject
}

type event struct {
	RelativeTime time.Duration
	eventDetails
}

var _ gojay.MarshalerJSONObject = event{}

func (e event) IsNil() bool { return false }
func (e event) MarshalJSONObject(enc *gojay.Encoder) {
	enc.Float64Key("time", milliseconds(e.RelativeTime))
	enc.StringKey("name", e.Category().String()+":"+e.Name())
	enc.ObjectKey("data", e.eventDetails)
}

type eventGeneric struct {
	name string
	msg  string
}

func (e eventGeneric) Category() category { return categoryGeneric }
func (e eventGeneric) Name() string       { return e.name }
func (e eventGeneric) IsNil() bool        { return false }

func (e eventGeneric) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("details", e.msg)
}

type metrics struct {
	MinRTT      time.Duration
	SmoothedRTT time.Duration
	LatestRTT   time.Duration
	RTTVariance time.Duration
}

type eventMetricsUpdated struct {
	Last    *metrics
	Current *metrics
}

func (e eventMetricsUpdated) Category() category { return categoryGeneric }
func (e eventMetricsUpdated) Name() string       { return "metrics_updated" }
func (e eventMetricsUpdated) IsNil() bool        { return false }

func (e eventMetricsUpdated) MarshalJSONObject(enc *gojay.Encoder) {
	if e.Last == nil || e.Last.MinRTT != e.Current.MinRTT {
		enc.FloatKey("min_rtt", milliseconds(e.Current.MinRTT))
	}
	if e.Last == nil || e.Last.SmoothedRTT != e.Current.SmoothedRTT {
		enc.FloatKey("smoothed_rtt", milliseconds(e.Current.SmoothedRTT))
	}
	if e.Last == nil || e.Last.LatestRTT != e.Current.LatestRTT {
		enc.FloatKey("latest_rtt", milliseconds(e.Current.LatestRTT))
	}
	if e.Last == nil || e.Last.RTTVariance != e.Current.RTTVariance {
		enc.FloatKey("rtt_variance", milliseconds(e.Current.RTTVariance))
	}
}

// Playback

// ABR

// Buffer

type eventBufferOccupancyUpdated struct {
	media_type   MediaType
	buffer_stats bufferStats
}

func (e eventBufferOccupancyUpdated) Category() category { return categoryBuffer }
func (e eventBufferOccupancyUpdated) Name() string       { return "occupancy_update" }
func (e eventBufferOccupancyUpdated) IsNil() bool        { return false }

func (e eventBufferOccupancyUpdated) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("media_type", e.media_type.String())
	enc.Int64Key("playout_ms", e.buffer_stats.PlayoutTime.Milliseconds())
	if e.buffer_stats.PlayoutBytes >= 0 {
		enc.Int64Key("playout_bytes", e.buffer_stats.PlayoutBytes)
	}
	if e.buffer_stats.PlayoutFrames >= 0 {
		enc.Int64Key("playout_frames", int64(e.buffer_stats.PlayoutFrames))
	}

	enc.Int64Key("max_ms", e.buffer_stats.MaxTime.Milliseconds())
	if e.buffer_stats.MaxBytes >= 0 {
		enc.Int64Key("max_bytes", e.buffer_stats.MaxBytes)
	}
	if e.buffer_stats.MaxFrames >= 0 {
		enc.Int64Key("max_frames", int64(e.buffer_stats.MaxFrames))
	}
}

// Network

type eventNetworkRequest struct {
	media_type   MediaType
	resource_url string
	byte_range   string
}

func (e eventNetworkRequest) Category() category { return categoryNetwork }
func (e eventNetworkRequest) Name() string       { return "request" }
func (e eventNetworkRequest) IsNil() bool        { return false }

func (e eventNetworkRequest) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("media_type", e.media_type.String())
	enc.StringKey("resource_url", e.resource_url)
	enc.StringKeyOmitEmpty("range", e.byte_range)
}
