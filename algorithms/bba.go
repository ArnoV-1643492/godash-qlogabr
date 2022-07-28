/*
* Arno Verstraete
 */

package algorithms

import (
	"fmt"

	"github.com/uccmisl/godash/logging"
)

/*
* Selects the representation index according to the BBA algorithm
 */
func BBA(bufferLevel_Milliseconds int, maxBufferLevel_Seconds int, highestMPDrepRateIndex int, lowestMPDrepRateIndex int, bandwithList []int,
	segmentDuration int, debugLog bool, debugFile string, thrList *[]int, newThr int) int {

	*thrList = append(*thrList, newThr)

	maxBufferLevel_Milliseconds := maxBufferLevel_Seconds * 1000

	// Static reservoirs
	var reservoir_lower float64 = 0.1 * float64(maxBufferLevel_Milliseconds)
	var reservoir_upper float64 = reservoir_lower

	// If this statement hits there only fits one segment in the reservoir, which could be too little
	if debugLog && segmentDuration > int(reservoir_lower)/2 {
		logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "The buffer is relatively small for the current segment duration")
	}

	// If we are in the lower reservoir, select the lowest bandwith
	if reservoir_lower >= float64(bufferLevel_Milliseconds) {
		fmt.Println("In lower reservoir")
		return lowestMPDrepRateIndex
	} else if maxBufferLevel_Milliseconds-int(reservoir_upper) <= bufferLevel_Milliseconds {
		// If we are in the higher reservoir, select the highest bandwixth
		fmt.Println("In upper reservoir")
		return highestMPDrepRateIndex
	}

	// Available representation boundaries
	R1 := LowestBitrate(bandwithList)
	Rmax := HighestBitrate(bandwithList)

	// Buffer cushion boundaries
	//B1 := reservoir_lower
	Bm := float64(maxBufferLevel_Milliseconds) - reservoir_lower - reservoir_upper

	// Buffer percentage
	var percentage float64 = (float64(bufferLevel_Milliseconds) / Bm)

	fmt.Println(percentage, bufferLevel_Milliseconds, Bm)

	// Map to a bitrate
	var desiredBitrate float64 = (percentage * Rmax) + R1

	// Choose the representation that best fits this bitrate
	chosenRep := SelectRepRateWithThroughtput(int(desiredBitrate), bandwithList, lowestMPDrepRateIndex)

	return chosenRep
}
