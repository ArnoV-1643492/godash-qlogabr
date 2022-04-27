package crosslayer

import (
	"fmt"
	"time"

	"github.com/lucas-clemente/quic-go/qlog"
)

type CrossLayerAccountant struct {
	EventChannel          chan qlog.Event
	throughputList        []float64
	relativeTimeLastEvent time.Duration
}

func (a CrossLayerAccountant) Listen(trackEvents bool) {
	go a.channelListenerThread(trackEvents)
}

func (a CrossLayerAccountant) channelListenerThread(trackEvents bool) {
	for msg := range a.EventChannel {
		// Only process events when this bool is set
		if trackEvents {
			a.relativeTimeLastEvent = msg.RelativeTime
			details := msg.GetEventDetails()
			eventType := details.EventType()
			if eventType == "EventPacketReceived" {
				fmt.Println(eventType)
				packetReceivedPointer := details.(*qlog.EventPacketReceived)
				packetReceived := *packetReceivedPointer
				fmt.Println(packetReceived.Length)
				a.throughputList = append(a.throughputList, float64(packetReceived.Length))
			}
		}
	}
}

/**
* Returns average measured throughput in bytes/second
 */
func (a CrossLayerAccountant) GetAverageThroughput() float64 {
	// Calculate sum
	var sum float64 = 0
	for _, el := range a.throughputList {
		sum += el
	}
	return sum / a.relativeTimeLastEvent.Seconds()
}
