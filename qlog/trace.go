package qlog

import (
	"time"

	"github.com/francoispqt/gojay"
)

type topLevel struct {
	trace trace
}

func (topLevel) IsNil() bool { return false }
func (l topLevel) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("qlog_format", "NDJSON")
	enc.StringKey("qlog_version", "draft-02")
	enc.StringKeyOmitEmpty("title", "qlog-abr")
	enc.StringKey("code_version", goDashVersion)
	enc.ObjectKey("trace", l.trace)
}

type vantagePoint struct {
	Name string
	Type Perspective
}

func (p vantagePoint) IsNil() bool { return false }
func (p vantagePoint) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKeyOmitEmpty("name", p.Name)
	switch p.Type {
	case PerspectiveClient:
		enc.StringKey("type", "client")
	case PerspectiveServer:
		enc.StringKey("type", "server")
	}
}

type commonFields struct {
	ProtocolType  string
	ReferenceTime time.Time
}

func (f commonFields) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKeyOmitEmpty("protocol_type", f.ProtocolType)
	enc.Float64Key("reference_time", float64(f.ReferenceTime.UnixNano())/1e6)
	enc.StringKey("time_format", "relative")
}

func (f commonFields) IsNil() bool { return false }

type trace struct {
	VantagePoint vantagePoint
	CommonFields commonFields
}

func (trace) IsNil() bool { return false }
func (t trace) MarshalJSONObject(enc *gojay.Encoder) {
	enc.ObjectKey("vantage_point", t.VantagePoint)
	enc.ObjectKey("common_fields", t.CommonFields)
}
