package crosslayer

import (
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go/qlog"
)

type CrossLayerAccountant struct {
	EventChannel          chan qlog.Event
	throughputList        []float64
	relativeTimeLastEvent time.Duration
	mu                    sync.Mutex
}

func (a *CrossLayerAccountant) Listen(trackEvents bool) {
	go a.channelListenerThread(trackEvents)
}

func (a *CrossLayerAccountant) channelListenerThread(trackEvents bool) {
	for msg := range a.EventChannel {
		// Only process events when this bool is set
		if trackEvents {
			a.relativeTimeLastEvent = msg.RelativeTime
			details := msg.GetEventDetails()
			eventType := details.EventType()
			if eventType == "EventPacketReceived" {
				//fmt.Println(eventType)
				packetReceivedPointer := details.(*qlog.EventPacketReceived)
				packetReceived := *packetReceivedPointer
				//fmt.Println(packetReceived.Length)
				a.mu.Lock()
				a.throughputList = append(a.throughputList, float64(packetReceived.Length))
				a.mu.Unlock()
				//fmt.Println(len(a.throughputList))
			}
		}
	}
}

/**
* Returns average measured throughput in bytes/second
 */
func (a *CrossLayerAccountant) GetAverageThroughput() float64 {
	// Calculate sum
	//fmt.Println(len(a.throughputList))
	var sum float64 = 0
	for _, el := range a.throughputList {
		sum += el
	}
	return sum / a.relativeTimeLastEvent.Seconds()
}
