package crosslayer

import (
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
		details := msg.GetEventDetails()
		eventName := details.Name()
		if eventName == "packet_received" {
			//qlog.eventPacketReceived(details)
		}

		/*msgJSON, _ := gojay.Marshal(msg)
		msgStr := string(msgJSON)
		var reading map[string]interface{}
		err := json.Unmarshal([]byte(msgStr), &reading)
		if err != nil {
			fmt.Println(err)
		} else {
			//fmt.Println(reading["name"])
			if reading["name"] == "transport:packet_received" {
				fmt.Println(reading["name"])
				fmt.Println(reading["data"].(map[string]interface{})["raw"].(map[string]interface{})["length"].(string))
			}
		}
		//var result map[string]interface{}
		//gojay.Unmarshal(msgJSON, &result)
		/*msgTime := msg.RelativeTime.Seconds()
		msgType := msg.Name()
		if msgType == "packet_received" {
			//msgSize := msg.eventDetails.Payload
			payloadlen := msg.GetEventPayloadLength()
			if payloadlen != 0 {
				fmt.Println(msgTime)
				fmt.Println(payloadlen)
			}
		}*/
	}
}
