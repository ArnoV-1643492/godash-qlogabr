package crosslayer

import (
	"context"
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

	// Variables for stall prediction
	predictStall                              bool
	bufferLevel_atStartOfSegment_Milliseconds int
	representationBitrate                     int // kbits / second
	segmentDuration_Milliseconds              int
	predictionWindow                          int         // number of packets the predictor looks at
	arrivalTimes                              []time.Time // List of the arrival times of each packet in throughputList
	time_atStartOfSegment                     time.Time
	m_cancelctx                               context.Context // Is called when the HTTP request needs to be cancelled
}

func (a *CrossLayerAccountant) InitialisePredictor() {
	fmt.Println("Stall prediction enabled")
	a.predictionWindow = 100
	a.predictStall = true
}

func (a *CrossLayerAccountant) SegmentStart_predictStall(segDuration_ms int, repLevel_kbps int, currBufferLevel int, cancelctx context.Context) {
	a.m_cancelctx = cancelctx
	a.StartTiming()
	a.bufferLevel_atStartOfSegment_Milliseconds = currBufferLevel
	a.time_atStartOfSegment = time.Now()
	//fmt.Println("PREDICTORBUFFER: ", a.bufferLevel_atStartOfSegment_Milliseconds)

	// Empty the throughput and timing lists
	a.mu.Lock()
	//fmt.Println("NUMBEROFPACKETS: ", len(a.throughputList))
	a.throughputList = nil
	a.arrivalTimes = nil
	a.mu.Unlock()

	a.segmentDuration_Milliseconds = segDuration_ms
	a.representationBitrate = repLevel_kbps
}

func (a *CrossLayerAccountant) SetTrackingEvents(trackEvents bool) {
	a.trackEvents = trackEvents
}

func (a *CrossLayerAccountant) Listen(trackEvents bool) {
	a.totalPassed_ms = 0

	a.SetTrackingEvents(trackEvents)
	go a.channelListenerThread()
}

func (a *CrossLayerAccountant) stallPredictor() {
	// Only do predictions when we have received enough packets
	if len(a.throughputList) > a.predictionWindow {
		//fmt.Println("IN PREDICTION WINDOW")
		a.mu.Lock()

		// Calculate sum of all bits received
		var sliceOfList []int = a.throughputList[len(a.throughputList)-a.predictionWindow:]
		var sum int = 0
		for _, el := range sliceOfList {
			sum += el
		}
		sum_bits := sum * 8
		predictionWindowStartTime := a.arrivalTimes[len(a.throughputList)-a.predictionWindow]

		// Calculate the average throughput of the prediction window
		windowTotalTime_ms := time.Since(predictionWindowStartTime).Milliseconds()

		a.mu.Unlock()

		// bits 	:=    (kbps == bpms)   		 / ms
		segmentSize := (a.representationBitrate) / a.segmentDuration_Milliseconds

		// Only do predictions when we have received less bytes than we expect to receive
		if sum_bits < segmentSize && windowTotalTime_ms > 0 {
			bitsToDownload := segmentSize - sum_bits // Number of bytes that need to be downloaded
			// bits / ms  := bits / ms
			windowBitrate := sum_bits / int(windowTotalTime_ms)
			// Time it will take in ms to download the remaining bits at this rate
			requiredTime_ms := bitsToDownload / windowBitrate

			if requiredTime_ms > a.calculateCurrentBufferLevel() {
				// Report stall prediction
				fmt.Println("STALLPREDICTOR ", time.Now().UnixMilli())
				a.m_cancelctx.Done()
			} else {
				fmt.Println("NO STALL", requiredTime_ms, a.calculateCurrentBufferLevel())
			}
		}
	}
}

func (a *CrossLayerAccountant) calculateCurrentBufferLevel() int {
	passedTime := time.Since(a.time_atStartOfSegment).Milliseconds()
	level := a.bufferLevel_atStartOfSegment_Milliseconds - int(passedTime)

	// Buffer cannot go below 0
	if level < 0 {
		level = 0
	}

	fmt.Println("CROSSLAYERBUFFERLEVEL", level, time.Now().UnixMilli())

	return level
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

				// If we are doing stall predictions, calculate prediction after this packet is received
				if a.predictStall {
					// Measure arrival time as well
					a.mu.Lock()
					a.arrivalTimes = append(a.arrivalTimes, time.Now())
					a.mu.Unlock()

					a.stallPredictor()
				}
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

/**
* Returns average measured throughput in bits/second of last 3000 packets
 */
func (a *CrossLayerAccountant) GetRecentAverageThroughput() float64 {
	// Calculate sum
	var sum int = 0
	if len(a.throughputList) > 3000 {
		var sliceOfList []int = a.throughputList[len(a.throughputList)-3000 : len(a.throughputList)+1]
		for _, el := range sliceOfList {
			sum += el
		}
	} else {
		for _, el := range a.throughputList {
			sum += el
		}
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
