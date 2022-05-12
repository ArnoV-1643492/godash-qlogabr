package crosslayer

import (
	"fmt"
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go/qlog"
)

type CrossLayerAccountant struct {
	EventChannel   chan qlog.Event
	throughputList []int // list of bytes
	//relativeTimeLastEvent time.Duration
	mu          sync.Mutex
	trackEvents bool

	// Variables used for tracking elapsed time
	totalPassed_ms  int64
	currentlyTiming bool // Indicates if we are currently downloading and tracking time
	currStartTime   time.Time
}

func (a *CrossLayerAccountant) SetTrackingEvents(trackEvents bool) {
	a.trackEvents = trackEvents
}

func (a *CrossLayerAccountant) Listen(trackEvents bool) {
	a.totalPassed_ms = 0

	a.SetTrackingEvents(trackEvents)
	go a.channelListenerThread()
}

func (a *CrossLayerAccountant) channelListenerThread() {
	for msg := range a.EventChannel {
		// Only process events when this bool is set
		if a.trackEvents {
			//a.relativeTimeLastEvent = msg.RelativeTime
			details := msg.GetEventDetails()
			eventType := details.EventType()
			if eventType == "EventPacketReceived" {
				//fmt.Println(eventType)
				packetReceivedPointer := details.(*qlog.EventPacketReceived)
				//fmt.Println(packetReceivedPointer.Length)
				a.mu.Lock()
				a.throughputList = append(a.throughputList, int(packetReceivedPointer.Length))
				a.mu.Unlock()
			}
		}
	}
}

/**
* Returns average measured throughput in bits/second
 */
func (a *CrossLayerAccountant) GetAverageThroughput() float64 {
	// Calculate sum
	var sum int = 0
	for _, el := range a.throughputList {
		sum += el
	}
	/*
		fmt.Println("Sum XL: ", sum)
			f, err := os.OpenFile("/tmp/trace.csv", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
			if err != nil {
				fmt.Println(err)
			} else {
				f.WriteString(strconv.FormatInt(int64(sum), 10) + "\n")
			}*/
	//     bits			  /  second
	return float64(sum*8) / (float64(a.getTotalTime()) / 1000) // convert it to seconds
}

// Should be called when we start downloading a segment
func (a *CrossLayerAccountant) StartTiming() {
	a.currStartTime = time.Now()
	a.currentlyTiming = true
}

// Should be called when we have received an entire segment
func (a *CrossLayerAccountant) StopTiming() int {
	if a.currentlyTiming {
		currPassedTime := time.Since(a.currStartTime)
		currPassedTime_ms := currPassedTime.Milliseconds()

		a.totalPassed_ms += currPassedTime_ms
		a.currentlyTiming = false
		return int(currPassedTime_ms)
	} else {
		fmt.Printf("Warning: stopping timer while timer is not running")
		return 0
	}
}

// Returns the total measured time in miliseconds, even when the current timer is still running
func (a *CrossLayerAccountant) getTotalTime() int64 {
	// If we are currently timing, calculate the current passed time and add it to the total before returning
	if a.currentlyTiming {
		currPassedTime := time.Since(a.currStartTime)
		currPassedTime_ms := currPassedTime.Milliseconds()
		return a.totalPassed_ms + currPassedTime_ms
	} else {
		return a.totalPassed_ms
	}
}
