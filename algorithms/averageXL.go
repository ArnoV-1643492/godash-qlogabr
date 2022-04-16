/**

Cross-layer version of the MeanAverage algorithm

*/

package algorithms

import (
	"fmt"

	"github.com/uccmisl/godash/crosslayer"
)

func MeanAverageXLAlgo(XLaccountant crosslayer.CrossLayerAccountant, thrList *[]int, newThr int, repRate *int, bandwithList []int, lowestMPDrepRateIndex int) {
	var average float64

	*thrList = append(*thrList, newThr)

	//if there is not enough throughtputs in the list, can't calculate the average
	if len(*thrList) < 2 {
		//if there is not enough throughtput, we call selectRepRate() with the newThr
		*repRate = SelectRepRateWithThroughtput(newThr, bandwithList, lowestMPDrepRateIndex)
		return
	}

	// average of the last throughtputs
	meanAverage(*thrList, &average)

	fmt.Println("AVERAGE: ", int(average))
	fmt.Println("AVERAGEXL: ", int(XLaccountant.GetAverageThroughput()))

	//We select the reprate with the calculated throughtput
	*repRate = SelectRepRateWithThroughtput(int(average), bandwithList, lowestMPDrepRateIndex)
}
