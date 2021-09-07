package qlog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"runtime/debug"
	"sync"
	"time"

	"github.com/francoispqt/gojay"
)

// Setting of this only works when quic-go is used as a library.
// When building a binary from this repository, the version can be set using the following go build flag:
// -ldflags="-X github.com/lucas-clemente/quic-go/qlog.quicGoVersion=foobar"
var goDashVersion = "(devel)"

func init() {
	if goDashVersion != "(devel)" { // variable set by ldflags
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok { // no build info available. This happens when quic-go is not used as a library.
		return
	}
	for _, d := range info.Deps {
		if d.Path == "github.com/uccmisl/godash" {
			goDashVersion = d.Version
			if d.Replace != nil {
				if len(d.Replace.Version) > 0 {
					goDashVersion = d.Version
				} else {
					goDashVersion += " (replaced)"
				}
			}
			break
		}
	}
}

const eventChanSize = 50

type Tracer struct {
	getLogWriter func(p Perspective, streamID string) io.WriteCloser
}

// NewTracer creates a new qlog tracer.
func NewTracer(getLogWriter func(p Perspective, streamID string) io.WriteCloser) *Tracer {
	return &Tracer{getLogWriter: getLogWriter}
}

func (t *Tracer) TracerForStream(_ context.Context, p Perspective, sid StreamID) *StreamTracer {
	if w := t.getLogWriter(p, sid.String()); w != nil {
		return NewStreamTracer(w, p, sid)
	}
	return nil
}

// A ConnectionTracer records events.
type streamTracer interface {
	// StartedConnection(local, remote net.Addr, srcConnID, destConnID ConnectionID)
	// NegotiatedVersion(chosen VersionNumber, clientVersions, serverVersions []VersionNumber)
	// ClosedConnection(error)
	// SentTransportParameters(*TransportParameters)
	// ReceivedTransportParameters(*TransportParameters)
	// RestoredTransportParameters(parameters *TransportParameters) // for 0-RTT
	// SentPacket(hdr *ExtendedHeader, size ByteCount, ack *AckFrame, frames []Frame)
	// ReceivedVersionNegotiationPacket(*Header, []VersionNumber)
	// ReceivedRetry(*Header)
	// ReceivedPacket(hdr *ExtendedHeader, size ByteCount, frames []Frame)
	// BufferedPacket(PacketType)
	// DroppedPacket(PacketType, ByteCount, PacketDropReason)
	UpdatedMetrics(rttStats *RTTStats)
	// AcknowledgedPacket(EncryptionLevel, PacketNumber)
	// LostPacket(EncryptionLevel, PacketNumber, PacketLossReason)
	// UpdatedCongestionState(CongestionState)
	// UpdatedPTOCount(value uint32)
	// UpdatedKeyFromTLS(EncryptionLevel, Perspective)
	// UpdatedKey(generation KeyPhase, remote bool)
	// DroppedEncryptionLevel(EncryptionLevel)
	// DroppedKey(generation KeyPhase)
	// SetLossTimer(TimerType, EncryptionLevel, time.Time)
	// LossTimerExpired(TimerType, EncryptionLevel)
	// LossTimerCanceled()
	// // Close is called when the connection is closed.
	Close()
	Debug(name, msg string)
}

type StreamTracer struct {
	mutex sync.Mutex

	w             io.WriteCloser
	sid           StreamID
	perspective   Perspective
	referenceTime time.Time

	events     chan event
	encodeErr  error
	runStopped chan struct{}

	RTT         *RTTStats
	lastMetrics *metrics
}

var _ streamTracer = &StreamTracer{}

func NewStreamTracer(w io.WriteCloser, p Perspective, sid StreamID) *StreamTracer {
	t := &StreamTracer{
		w:             w,
		perspective:   p,
		sid:           sid,
		runStopped:    make(chan struct{}),
		events:        make(chan event, eventChanSize),
		referenceTime: time.Now(),
		RTT:           NewRTTStats(),
	}
	go t.run()
	return t
}

func (t *StreamTracer) run() {
	defer close(t.runStopped)
	buf := &bytes.Buffer{}
	enc := gojay.NewEncoder(buf)
	tl := &topLevel{
		trace: trace{
			VantagePoint: vantagePoint{Type: t.perspective},
			CommonFields: commonFields{
				ReferenceTime: t.referenceTime,
			},
		},
	}
	if err := enc.Encode(tl); err != nil {
		panic(fmt.Sprintf("qlog encoding into a bytes.Buffer failed: %s", err))
	}
	if err := buf.WriteByte('\n'); err != nil {
		panic(fmt.Sprintf("qlog encoding into a bytes.Buffer failed: %s", err))
	}
	if _, err := t.w.Write(buf.Bytes()); err != nil {
		t.encodeErr = err
	}
	enc = gojay.NewEncoder(t.w)
	for ev := range t.events {
		if t.encodeErr != nil { // if encoding failed, just continue draining the event channel
			continue
		}
		if err := enc.Encode(ev); err != nil {
			t.encodeErr = err
			continue
		}
		if _, err := t.w.Write([]byte{'\n'}); err != nil {
			t.encodeErr = err
		}
	}
}

func (t *StreamTracer) Close() {
	if err := t.export(); err != nil {
		log.Printf("exporting qlog failed: %s\n", err)
	}
}

// export writes a qlog.
func (t *StreamTracer) export() error {
	close(t.events)
	<-t.runStopped
	if t.encodeErr != nil {
		return t.encodeErr
	}
	return t.w.Close()
}

func (t *StreamTracer) recordEvent(eventTime time.Time, details eventDetails) {
	t.events <- event{
		RelativeTime: eventTime.Sub(t.referenceTime),
		eventDetails: details,
	}
}

func (t *StreamTracer) Debug(name, msg string) {
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventGeneric{
		name: name,
		msg:  msg,
	})
	t.mutex.Unlock()
}

func (t *StreamTracer) UpdatedMetrics(rttStats *RTTStats) {
	m := &metrics{
		MinRTT:      rttStats.MinRTT(),
		SmoothedRTT: rttStats.SmoothedRTT(),
		LatestRTT:   rttStats.LatestRTT(),
		RTTVariance: rttStats.MeanDeviation(),
	}
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventMetricsUpdated{
		Last:    t.lastMetrics,
		Current: m,
	})
	t.lastMetrics = m
	t.mutex.Unlock()
}

// Playback

// ABR

// Buffer

func (t *StreamTracer) UpdateBufferOccupancy(mediaType MediaType, bufferStats bufferStats) {
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventBufferOccupancyUpdated{media_type: mediaType, buffer_stats: bufferStats})
	t.mutex.Unlock()
}

// Network

func (t *StreamTracer) Request(mediaType MediaType, resourceURL string, byteRange string) {
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventNetworkRequest{media_type: mediaType, resource_url: resourceURL, byte_range: byteRange})
	t.mutex.Unlock()
}
