package crosslayer

import (
	"fmt"

	"github.com/lucas-clemente/quic-go/qlog"
)

type CrossLayerAccountant struct {
	EventChannel   chan qlog.Event
	throughputList []float64
}

func (a CrossLayerAccountant) Listen() {
	go a.channelListenerThread()
}

func (a CrossLayerAccountant) channelListenerThread() {
	for msg := range a.EventChannel {
		//msgJSON, _ := gojay.Marshal(msg)
		//var result map[string]interface{}
		//gojay.Unmarshal(msgJSON, &result)
		msgTime := msg.RelativeTime.Seconds()
		msgType := msg.Name()
		if msgType == "packet_received" {
			//msgSize := msg.eventDetails.Payload
			fmt.Println(msgType)
			fmt.Println(msgTime)
		}
	}
}
