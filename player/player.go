/*
 *	goDASH, golang client emulator for DASH video streaming
 *	Copyright (c) 2019, Jason Quinlan, Darijo Raca, University College Cork
 *											[j.quinlan,d.raca]@cs.ucc.ie)
 *                      Maëlle Manifacier, MISL Summer of Code 2019, UCC
 *	This program is free software; you can redistribute it and/or
 *	modify it under the terms of the GNU General Public License
 *	as published by the Free Software Foundation; either version 2
 *	of the License, or (at your option) any later version.
 *
 *	This program is distributed in the hope that it will be useful,
 *	but WITHOUT ANY WARRANTY; without even the implied warranty of
 *	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *	GNU General Public License for more details.
 *
 *	You should have received a copy of the GNU General Public License
 *	along with this program; if not, write to the Free Software
 *	Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA
 *	02110-1301, USA.
 */

// ./goDASH -url "[http://cs1dev.ucc.ie/misl/4K_non_copyright_dataset/10_sec/x264/bbb/DASH_Files/full/bbb_enc_x264_dash.mpd, http://cs1dev.ucc.ie/misl/4K_non_copyright_dataset/10_sec/x264/bbb/DASH_Files/full/bbb_enc_x264_dash.mpd]" -adapt default -codec AVC -debug true -initBuffer 2 -maxBuffer 10 -maxHeight 1080 -numSegments 20  -storeDASH 347985

package player

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/uccmisl/godash/P2Pconsul"
	algo "github.com/uccmisl/godash/algorithms"
	glob "github.com/uccmisl/godash/global"
	"github.com/uccmisl/godash/hlsfunc"
	"github.com/uccmisl/godash/http"
	"github.com/uccmisl/godash/logging"
	"github.com/uccmisl/godash/qoe"
	"github.com/uccmisl/godash/utils"
)

// play position
var playPosition = 0

// current segment number
var segmentNumber = 1
var segmentDuration int
var nextSegmentNumber int

// current buffer level
var bufferLevel = 0
var maxBufferLevel int
var waitToPlayCounter = 0
var stallTime = 0

// current mpd file
var mpdListIndex = 0
var mpdListIndexArray []int
var lowestMPDrepRateIndex int
var highestMPDrepRateIndex int

// save the previous mpdIndex
var oldMPDIndex = 0

// determine if an MPD is byte-range or not
var isByteRangeMPD bool
var startRange = 0
var endRange = 0

// current representation rate
var repRate = 0

//var repRatesReversed bool

// current adaptationSet
var currentMPDRepAdaptSet int

// Segment size (in bits)
var segSize int

// baseURL for this MPD file
var baseURL string
var headerURL string
var currentURL string

// we need to keep a tab on the different size segments - use this for now
// we will use an array in the future
var segmentDurationTotal = 0
var segmentDurationArray []int

// the list of bandwith values (rep_rates) from the current MPD file
var bandwithList []int

// list of throughtputs - noted from downloading the segments
var thrList []int

// time values
var startTime time.Time
var nextRunTime time.Time
var arrivalTime int

// additional output logs values
var repCodec string
var repHeight int
var repWidth int
var repFps int
var mimeType string

// used to calculate targetRate - float64
var kP = 0.01
var kI = 0.001
var staticAlgParameter = 0.0

// first step is to check the first MPD for the codec (I had problem passing a
// 2-dimensional array, so I moved the check to here)
var codecList [][]string
var codecIndexList [][]int
var usedVideoCodec bool
var codecIndex int
var audioContent bool
var onlyAudio bool
var audioRate int
var audioCodec string

var urlInput []string

// For the mapSegments of segments :
// Map with the segment number and a structure of informations
// one map contains all content
var mapSegmentLogPrintouts []map[int]logging.SegPrintLogInformation

// a map of maps containing segment header information
var segHeadValues map[int]map[int][]int

// default value for the exponential ratio
var exponentialRatio float64

// file download location
var fileDownloadLocation string

// printHeadersData local
var printHeadersData map[string]string

// print the log to terminal
var printLog bool

// variable to determine if we are using the goDASHbed testbed
var useTestbedBool bool

// variable to determine if we should generate QoE values
var getQoEBool bool

// variable to determine if we should save our streaming files
var saveFilesBool bool

// other QoE variables
var segRates []float64
var sumSegRate float64
var totalStallDur float64
var nStalls int
var nSwitches int
var rateChange []float64
var sumRateChange float64
var rateDifference float64

// index values for the types of MPD types
var mimeTypes []int

var streamStructs []http.StreamStruct

// stop the player
var stopPlayer = true

// GetFullStreamHeader :
/*
 * get the init header file for this MPD url
 */
func GetFullStreamHeader(currentURL string, baseURL string, headerURL string, debugLog bool, Noden P2Pconsul.NodeUrl, segmentDuration int, profile string, debugFile string, adapt string, fileDownloadLocation string, startRange int, endRange int, segmentNumber int, quicBool bool, useTestbedBool bool, repRate int, saveFilesBool bool, AudioByteRange bool) (OriginalURL string, OriginalbaseURL string) {

	// Collaborative Code - Start
	OriginalURL = currentURL
	OriginalbaseURL = baseURL
	baseJoined := baseURL + headerURL
	urlHeaderString := http.JoinURL(currentURL, baseURL+headerURL, debugLog)
	if Noden.ClientName != glob.CollabPrintOff && Noden.ClientName != "" {
		currentURL = Noden.Search(urlHeaderString, segmentDuration, true, profile)

		logging.DebugPrint(debugFile, debugLog, "\nDEBUG: ", "current URL joined: "+currentURL)
		currentURL = strings.Split(currentURL, "::")[0]
		logging.DebugPrint(debugFile, debugLog, "\nDEBUG: ", "current URL joined: "+currentURL)
		urlSplit := strings.Split(currentURL, "/")
		logging.DebugPrint(debugFile, debugLog, "\nDEBUG: ", "current URL joined: "+urlSplit[len(urlSplit)-1])
		baseJoined = urlSplit[len(urlSplit)-1]
	}
	// Collaborative Code - End

	// determine the inital variables to set, based on the algorithm choice
	switch adapt {
	case glob.ConventionalAlg:
		// there is no byte range in this file, so we set byte-range bool to false
		// we don't want to add the seg duration to this file, so 'addSegDuration' is false
		http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
			segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
		// set the inital rep_rate to the lowest value index
		repRate = lowestMPDrepRateIndex
	case glob.ElasticAlg:
		http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
			segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
		repRate = lowestMPDrepRateIndex
	case glob.ProgressiveAlg:
		// get the header file
		// there is no byte range in this file, so we set byte-range bool to false
		http.GetFileProgressively(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber, segmentDuration, false, debugLog, AudioByteRange, profile)
	case glob.TestAlg:
		http.GetFile(currentURL, baseJoined, fileDownloadLocation, AudioByteRange, startRange, endRange, segmentNumber,
			segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
		repRate = lowestMPDrepRateIndex

	case glob.BBAAlg:
		http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
			segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)

		repRate = lowestMPDrepRateIndex

	case glob.ArbiterAlg:
		http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
			segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
		repRate = lowestMPDrepRateIndex

	case glob.LogisticAlg:
		http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
			segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
		repRate = lowestMPDrepRateIndex
	case glob.MeanAverageAlg:
		http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
			segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
	case glob.GeomAverageAlg:
		http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
			segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
	case glob.EMWAAverageAlg:
		http.GetFile(currentURL, baseJoined, fileDownloadLocation, false, startRange, endRange, segmentNumber,
			segmentDuration, true, quicBool, debugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
	}

	return
}

// Stream :
/*
 * get the header file for the current video clip
 * check the different arguments in order to stream
 * call streamLoop to begin to stream
 */
func Stream(mpdList []http.MPD, debugFile string, debugLog bool, codec string, codecName string, maxHeight int, streamDuration int, maxBuffer int, initBuffer int, adapt string, urlString string, fileDownloadLocationIn string, extendPrintLog bool, hls string, hlsBool bool, quic string, quicBool bool, getHeaderBool bool, getHeaderReadFromFile string, exponentialRatioIn float64, printHeadersDataIn map[string]string, printLogIn bool,
	useTestbedBoolIn bool, getQoEBoolIn bool, saveFilesBoolIn bool, Noden P2Pconsul.NodeUrl) {

	// set debug logs for the collab clients
	if Noden.ClientName != glob.CollabPrintOff && Noden.ClientName != "" {
		Noden.SetDebug(debugFile, debugLog)
	}

	// check if the codec is in the MPD urls passed in
	codecList, codecIndexList, audioContent = http.GetCodec(mpdList, codec, debugLog)
	// determine if the passed in codec is one of the codecs we use (checking the first MPD only)
	usedVideoCodec, codecIndex = utils.FindInStringArray(codecList[0], codec)

	// logs
	var mapSegmentLogPrintouts []map[int]logging.SegPrintLogInformation

	// set local val
	exponentialRatio = exponentialRatioIn
	fileDownloadLocation = fileDownloadLocationIn
	printHeadersData = printHeadersDataIn
	printLog = printLogIn
	useTestbedBool = useTestbedBoolIn
	getQoEBool = getQoEBoolIn
	saveFilesBool = saveFilesBoolIn

	// check the codec and print error is false
	// if !usedVideoCodec {
	// 	// print error message
	// 	fmt.Printf("*** -" + codecName + " " + codec + " is not in the first provided MPD, please check " + urlString + " ***\n")
	// 	// stop the app
	// 	utils.StopApp()
	// }
	if codecList[0][0] == glob.RepRateCodecAudio && len(codecList[0]) == 1 {
		logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "*** This is an audio only file, ignoring Video Codec - "+codec+" ***\n")
		onlyAudio = true
		// reset the codeIndex to suit Audio only
		codecIndex = 0
		//codecIndexList[0][codecIndex] = 0
	} else if !usedVideoCodec {
		// print error message
		logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "*** -"+glob.CodecName+" "+codec+" is not in the provided MPD, please check "+urlString+" ***\n")
		// stop the app
		utils.StopApp()
	}

	// the input must be a defined value - loops over the adaptationSets
	// currently one adaptation set per video and audio
	for currentMPDRepAdaptSetIndex := range codecIndexList[mpdListIndex] {

		// only use the selected input codec and audio (if audio exists)
		if codecIndexList[0][currentMPDRepAdaptSetIndex] != -1 {

			currentMPDRepAdaptSet = currentMPDRepAdaptSetIndex

			// lets work out how many mimeTypes we have
			mimeTypes = append(mimeTypes, currentMPDRepAdaptSetIndex)

			// store a variable for the MPD index we use for video/audio
			// start at zero
			mpdListIndexArray = append(mpdListIndexArray, 0)

			// currentMPDRepAdaptSet = 1
			// determine if we are using a byte-range or standard MPD profile
			// the xml Representation>BaseURL is saved in the same location
			// for byte range full, main and onDemand
			// so check for BaseURL, if not empty, then its a byte-range
			baseURL = http.GetRepresentationBaseURL(mpdList[mpdListIndex], 0)
			if baseURL != glob.RepRateBaseURL {
				isByteRangeMPD = true
				logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Byte-range MPD: ")
			}

			// get the relevent values from this MPD
			// maxSegments - number of segments to download
			// maxBufferLevel - maximum buffer level in seconds
			// highestMPDrepRateIndex - index with the highest rep_rate
			// lowestMPDrepRateIndex - index with the lowest rep_rate
			// segmentDuration - segment duration
			// bandwithList - get all the range of representation bandwiths of the MPD

			// maxSegments was the first value
			_, maxBufferLevel, highestMPDrepRateIndex, lowestMPDrepRateIndex, segmentDurationArray, bandwithList, baseURL = http.GetMPDValues(mpdList, mpdListIndex, maxHeight, streamDuration, maxBuffer, currentMPDRepAdaptSet, isByteRangeMPD, debugLog)

			// get the profile for this file
			profiles := strings.Split(mpdList[mpdListIndex].Profiles, ":")
			numProfile := len(profiles) - 2
			profile := profiles[numProfile]

			// if byte-range add this to the file name
			if isByteRangeMPD {
				profile += glob.ByteRangeString
			}

			logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "DASH profile for the header is: "+profile)

			// reset repRate
			repRate = lowestMPDrepRateIndex

			// print values to debug log
			logging.DebugPrint(debugFile, debugLog, "\nDEBUG: ", "streaming has begun")
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Input values to streaming algorithm: "+adapt)
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "maxHeight: "+strconv.Itoa(maxHeight))
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "streamDuration in seconds: "+strconv.Itoa(streamDuration))
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "maxBuffer: "+strconv.Itoa(maxBuffer))
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "initBuffer: "+strconv.Itoa(initBuffer))
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "url: "+urlString)
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "fileDownloadLocation: "+fileDownloadLocation)
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "HLS: "+hls)
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "extend: "+strconv.FormatBool(extendPrintLog))

			// update audio rate and codec
			AudioByteRange := false
			if audioContent && codecList[0][currentMPDRepAdaptSetIndex] == glob.RepRateCodecAudio {
				audioRate = mpdList[mpdListIndex].Periods[0].AdaptationSet[len(mimeTypes)-1].Representation[repRate].BandWidth / 1000
				audioCodec = mpdList[mpdListIndex].Periods[0].AdaptationSet[len(mimeTypes)-1].Representation[repRate].Codecs
				if isByteRangeMPD {
					AudioByteRange = true
					logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "Audio Byte-Range Header")
				}
			}

			// get the stream header from the required MPD (first index in the mpdList)
			headerURL = http.GetFullStreamHeader(mpdList[mpdListIndex], isByteRangeMPD, currentMPDRepAdaptSet, AudioByteRange, 0)
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "stream initialise URL header: "+headerURL)

			// convert the url strings to a list
			urlInput = http.URLList(urlString)

			// get the current url - trim any white space
			currentURL = strings.TrimSpace(urlInput[mpdListIndex])
			// currentURL := strings.TrimSpace(urlInput[mpdListIndex])
			logging.DebugPrint(debugFile, debugLog, "\nDEBUG: ", "current URL header: "+currentURL)

			// set the segmentDuration to the first passed in URL
			segmentDuration = segmentDurationArray[0]

			// get the init file for this MPD url
			OriginalURL, OriginalbaseURL := GetFullStreamHeader(currentURL, baseURL, headerURL, debugLog, Noden, segmentDuration, profile, debugFile, adapt, fileDownloadLocation, startRange, endRange, segmentNumber, quicBool, useTestbedBool, repRate, saveFilesBool, AudioByteRange)

			// determine the inital variables to set, based on the algorithm choice
			switch adapt {
			case glob.ConventionalAlg:
				repRate = lowestMPDrepRateIndex
			case glob.ElasticAlg:
				repRate = lowestMPDrepRateIndex
			case glob.TestAlg:
				repRate = lowestMPDrepRateIndex
			case glob.BBAAlg:
				repRate = lowestMPDrepRateIndex
			case glob.ArbiterAlg:
				repRate = lowestMPDrepRateIndex
			case glob.LogisticAlg:
				repRate = lowestMPDrepRateIndex
			}
			// debug logs
			logging.DebugPrint(debugFile, debugLog, "\nDEBUG: ", "We are using repRate: "+strconv.Itoa(repRate))
			logging.DebugPrint(debugFile, debugLog, "DEBUG: ", "We are using : "+adapt+" for streaming")

			//create the map for the print log
			var mapSegmentLogPrintout map[int]logging.SegPrintLogInformation
			mapSegmentLogPrintout = make(map[int]logging.SegPrintLogInformation)

			//StartTime of downloading
			startTime = time.Now()
			nextRunTime = time.Now()

			// get the segment headers and stop this run
			if getHeaderBool {
				// get the segment headers for all MPD url passed as arguments - print to file
				http.GetAllSegmentHeaders(mpdList, codecIndexList, maxHeight, 1, streamDuration, isByteRangeMPD, maxBuffer, headerURL, codec, urlInput, debugLog, true)

				// print error message
				fmt.Printf("*** - All segment header have been downloaded to " + glob.DebugFolder + " - ***\n")
				// exit the application
				os.Exit(3)
			} else {
				if getHeaderReadFromFile == glob.GetHeaderOnline {
					// get the segment headers for all MPD url passed as arguments - not from file
					segHeadValues = http.GetAllSegmentHeaders(mpdList, codecIndexList, maxHeight, 1, streamDuration, isByteRangeMPD, maxBuffer, headerURL, codec, urlInput, debugLog, false)
				} else if getHeaderReadFromFile == glob.GetHeaderOffline {
					// get the segment headers for all MPD url passed as arguments - yes from file
					// get headers from file for a given number of seconds of stream time
					// let's assume every n seconds
					segHeadValues = http.GetNSegmentHeaders(mpdList, codecIndexList, maxHeight, 1, streamDuration, isByteRangeMPD, maxBuffer, headerURL, codec, urlInput, debugLog, true)

				}
			}

			// I need to have two of more sets of lists for the following content
			streaminfo := http.StreamStruct{
				SegmentNumber:         segmentNumber,
				CurrentURL:            OriginalURL,
				InitBuffer:            initBuffer,
				MaxBuffer:             maxBuffer,
				CodecName:             codecName,
				Codec:                 codec,
				UrlString:             urlString,
				UrlInput:              urlInput,
				MpdList:               mpdList,
				Adapt:                 adapt,
				MaxHeight:             maxHeight,
				IsByteRangeMPD:        isByteRangeMPD,
				StartTime:             startTime,
				NextRunTime:           nextRunTime,
				ArrivalTime:           arrivalTime,
				OldMPDIndex:           0,
				NextSegmentNumber:     1,
				Hls:                   hls,
				HlsBool:               hlsBool,
				MapSegmentLogPrintout: mapSegmentLogPrintout,
				StreamDuration:        streamDuration,
				ExtendPrintLog:        extendPrintLog,
				HlsUsed:               false,
				BufferLevel:           bufferLevel,
				SegmentDurationTotal:  segmentDurationTotal,
				Quic:                  quic,
				QuicBool:              quicBool,
				BaseURL:               OriginalbaseURL,
				DebugLog:              debugLog,
				AudioContent:          audioContent,
				RepRate:               repRate,
				BandwithList:          bandwithList,
				Profile:               profile,
				LowestMPDrepRateIndex: lowestMPDrepRateIndex,
				WaitToPlayCounter:     waitToPlayCounter,
			}
			streamStructs = append(streamStructs, streaminfo)
			mapSegmentLogPrintouts = append(mapSegmentLogPrintouts, mapSegmentLogPrintout)
		}
	}

	// reset currentMPDRepAdaptSet
	// currentMPDRepAdaptSet = 0

	// print the output log headers
	logging.PrintHeaders(extendPrintLog, fileDownloadLocation, glob.LogDownload, debugFile, debugLog, printLog, printHeadersData)

	// Streaming loop function - using the first MPD index - 0, and hlsUsed false
	segmentNumber, mapSegmentLogPrintouts = streamLoop(streamStructs, Noden)

	// print sections of the map to the debug log - if debug is true
	if debugLog {
		logging.PrintsegInformationLogMap(debugFile, debugLog, mapSegmentLogPrintouts[0])
	}

	// print out the rest of the play out segments
	logging.PrintPlayOutLog(streamDuration*2, initBuffer, mapSegmentLogPrintouts, glob.LogDownload, printLog, printHeadersData)
}

// streamLoop :
/*
 * take the first segment number, download it with a low quality
 * call itself with the next segment number
 */
func streamLoop(streamStructs []http.StreamStruct, Noden P2Pconsul.NodeUrl) (int, []map[int]logging.SegPrintLogInformation) {

	// variable for rtt for this segment
	var rtt time.Duration
	// has this chunk been replaced by hls
	var hlsReplaced = "no"
	// if we undertake HLS, we need to revise the buffer values
	var bufferDifference int
	// if we set this chunk to HLS used
	if streamStructs[0].HlsUsed {
		hlsReplaced = "yes"
	}
	var segURL string

	// save point for the HTTP protocol used
	var protocol string

	//
	var segmentFileName string

	//
	var P1203Header float64

	// logging info
	// var mapSegmentLogPrintouts []map[int]logging.SegPrintLogInformation

	// lets loop over our mimeTypes
	for mimeTypeIndex := range mimeTypes {

		mpdListIndex := mpdListIndexArray[mimeTypeIndex]

		// get the values from the stream struct
		incrementalSegmentNumber := streamStructs[mimeTypeIndex].SegmentNumber
		currentURL := streamStructs[mimeTypeIndex].CurrentURL
		initBuffer := streamStructs[mimeTypeIndex].InitBuffer
		maxBuffer := streamStructs[mimeTypeIndex].MaxBuffer
		codecName := streamStructs[mimeTypeIndex].CodecName
		codec := streamStructs[mimeTypeIndex].Codec
		urlString := streamStructs[mimeTypeIndex].UrlString
		urlInput := streamStructs[mimeTypeIndex].UrlInput
		mpdList := streamStructs[mimeTypeIndex].MpdList
		adapt := streamStructs[mimeTypeIndex].Adapt
		maxHeight := streamStructs[mimeTypeIndex].MaxHeight
		isByteRangeMPD := streamStructs[mimeTypeIndex].IsByteRangeMPD
		startTime := streamStructs[mimeTypeIndex].StartTime
		nextRunTime := streamStructs[mimeTypeIndex].NextRunTime
		arrivalTime := streamStructs[mimeTypeIndex].ArrivalTime
		oldMPDIndex := streamStructs[mimeTypeIndex].OldMPDIndex
		nextSegmentNumber := streamStructs[mimeTypeIndex].NextSegmentNumber
		hls := streamStructs[mimeTypeIndex].Hls
		hlsBool := streamStructs[mimeTypeIndex].HlsBool
		mapSegmentLogPrintout := streamStructs[mimeTypeIndex].MapSegmentLogPrintout
		streamDuration := streamStructs[mimeTypeIndex].StreamDuration
		extendPrintLog := streamStructs[mimeTypeIndex].ExtendPrintLog
		hlsUsed := streamStructs[mimeTypeIndex].HlsUsed
		bufferLevel := streamStructs[mimeTypeIndex].BufferLevel
		segmentDurationTotal := streamStructs[mimeTypeIndex].SegmentDurationTotal
		quic := streamStructs[mimeTypeIndex].Quic
		quicBool := streamStructs[mimeTypeIndex].QuicBool
		baseURL := streamStructs[mimeTypeIndex].BaseURL
		debugLog := streamStructs[mimeTypeIndex].DebugLog
		audioContent := streamStructs[mimeTypeIndex].AudioContent
		repRate := streamStructs[mimeTypeIndex].RepRate
		bandwithList := streamStructs[mimeTypeIndex].BandwithList
		profile := streamStructs[mimeTypeIndex].Profile
		lowestMPDrepRateIndex :=
			streamStructs[mimeTypeIndex].LowestMPDrepRateIndex
		waitToPlayCounter := streamStructs[mimeTypeIndex].WaitToPlayCounter

		// update the segment number if we are moving between url indexes
		segmentNumber := incrementalSegmentNumber
		if incrementalSegmentNumber != nextSegmentNumber {
			segmentNumber = nextSegmentNumber
		}

		// determine the MimeType and mimeTypeIndex - set video by default
		// get the mimeType of this adaptationSet
		mimeType = mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypeIndex].Representation[repRate].MimeType

		// update audio rate and codec
		AudioByteRange := false
		if audioContent && mimeType == glob.RepRateCodecAudio {
			audioRate = mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].BandWidth / 1000
			audioCodec = mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].Codecs
			if isByteRangeMPD {
				AudioByteRange = true
				logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "Audio Byte-Range Segment")
			}
		}

		logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "current MimeType header: "+mimeType)
		/*
		 * Function  :
		 * let's think about HLS - chunk replacement
		 * before we decide what chunks to change, lets create a file for HLS
		 * then add functions to switch out an old chunk
		 */
		// only use HLS if we have at least one segment to replacement
		if hlsBool && segmentNumber > 1 &&
			mimeType == glob.RepRateCodecVideo {
			switch hls {
			// passive - least amount of replacement
			case glob.HlsOn:
				if segmentNumber == 6 {
					// hlsUsed is set to true
					chunkReplace := 5
					var thisRunTimeVal int
					// replace a previously downloaded segment with this call
					nextSegmentNumber, mapSegmentLogPrintouts, bufferDifference, thisRunTimeVal, nextRunTime =
						hlsfunc.GetHlsSegment(
							streamLoop,
							chunkReplace,
							mapSegmentLogPrintouts,
							maxHeight,
							urlInput,
							initBuffer,
							maxBuffer,
							codecName,
							codec,
							urlString,
							mpdList,
							nextSegmentNumber,
							extendPrintLog,
							startTime,
							nextRunTime,
							arrivalTime,
							true,
							quic,
							quicBool,
							baseURL,
							glob.DebugFile,
							debugLog,
							glob.RepRateBaseURL,
							audioContent,
							repRate,
							mimeTypeIndex,
							Noden,
						)

					// change the current buffer to reflect the time taken to get this HLS segment
					bufferLevel -= (thisRunTimeVal + bufferDifference)

					// change the buffer levels of the previous chunks, so the printout reflects this value
					mapSegmentLogPrintouts[mimeTypeIndex] = hlsfunc.ChangeBufferLevels(mapSegmentLogPrintouts[mimeTypeIndex], segmentNumber, chunkReplace, bufferDifference)
				}
			}
		}

		// if we have changed the MPD, we need to update some variables
		if oldMPDIndex != mpdListIndex {

			// set the new mpdListIndex
			mpdListIndex = oldMPDIndex
			mpdListIndexArray[mimeTypeIndex] = oldMPDIndex

			// get the current url - trim any white space
			currentURL = strings.TrimSpace(urlInput[mpdListIndex])
			logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "current URL header: "+currentURL)

			// get the relavent values from this MPD
			streamDuration, maxBufferLevel, highestMPDrepRateIndex, lowestMPDrepRateIndex, segmentDurationArray, bandwithList, baseURL = http.GetMPDValues(mpdList, mpdListIndex, maxHeight, streamDuration, maxBuffer, mimeTypes[mimeTypeIndex], isByteRangeMPD, debugLog)

			// current segment duration
			segmentDuration = segmentDurationArray[mpdListIndex]

			// ONLY CHANGE THE NUMBER OF SEGMENTS HERE
			//	numSegments := streamDuration / segmentDuration

			// determine if the passed in codec is one of the codecs we use (checking the current MPD)
			usedVideoCodec, codecIndex = utils.FindInStringArray(codecList[0], codec)
			// check the codec and print error is false
			// if !usedVideoCodec {
			// 	// print error message
			// 	fmt.Printf("*** -" + codecName + " " + codec + " is not in the provided MPD, please check " + urlString + " ***\n")
			// 	// stop the app
			// 	utils.StopApp()
			// }
			if codecList[0][0] == glob.RepRateCodecAudio && len(codecList[0]) == 1 {
				logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "*** This is an audio only file, ignoring Video Codec - "+codec+" ***\n")
				onlyAudio = true
				// reset the codeIndex to suit Audio only
				codecIndex = 0
				//codecIndexList[0][codecIndex] = 0
			} else if !usedVideoCodec {
				// print error message
				logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "*** -"+glob.CodecName+" "+codec+" is not in the provided MPD, please check "+urlString+" ***\n")
				// stop the app
				utils.StopApp()
			}

			// save the current MPD Rep_rate Adaptation Set
			mimeTypes[mimeTypeIndex] = mimeTypeIndex

			// get the profile for this file
			profiles := strings.Split(mpdList[mpdListIndex].Profiles, ":")
			numProfile := len(profiles) - 2
			profile = profiles[numProfile]

			// if byte-range add this to the file name
			if isByteRangeMPD {
				profile += glob.ByteRangeString
			}

			// get the init file for this MPD url
			headerURL = http.GetFullStreamHeader(mpdList[mpdListIndex], isByteRangeMPD, mimeTypes[mimeTypeIndex], AudioByteRange, 0)
			initNameSlice := strings.Split(headerURL, "/")
			fileCheck := "./" + fileDownloadLocation + "/" + strconv.Itoa(segmentDuration) + "sec_" + profile + "_" + initNameSlice[len(initNameSlice)-1]

			// only get the file if it does not exist
			if _, err := os.Stat(fileCheck); err != nil {

				GetFullStreamHeader(currentURL, baseURL, headerURL, debugLog, Noden, segmentDuration, profile, glob.DebugFile, adapt, fileDownloadLocation, startRange, endRange, segmentNumber, quicBool, useTestbedBool, repRate, saveFilesBool, AudioByteRange)
			}
		}
		logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "DASH profile for this segment is: "+profile)

		// break out if we have downloaded all of our segments
		// which is current segment duration total plus the next segment to be downloaded
		if segmentDurationTotal+(segmentDuration*glob.Conversion1000) > streamDuration &&
			mimeTypeIndex == len(mimeTypes)-1 {
			// save the current log
			streamStructs[mimeTypeIndex].MapSegmentLogPrintout = mapSegmentLogPrintout
			// get the logs for all adaptationSets
			for mimeTypeIndex := range mimeTypes {
				mapSegmentLogPrintouts = append(mapSegmentLogPrintouts, streamStructs[mimeTypeIndex].MapSegmentLogPrintout)
			}
			return segmentNumber, mapSegmentLogPrintouts
		}

		// keep rep_rate within the index boundaries
		// MISL - might cause problems
		if repRate < highestMPDrepRateIndex {
			logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "Changing rep_rate index: from "+strconv.Itoa(repRate)+" to "+strconv.Itoa(highestMPDrepRateIndex))
			repRate = highestMPDrepRateIndex
		}

		// get the segment
		if isByteRangeMPD {
			segURL, startRange, endRange = http.GetNextByteRangeURL(mpdList[mpdListIndex], segmentNumber, repRate, mimeTypes[mimeTypeIndex])
			logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "byte start range: "+strconv.Itoa(startRange))
			logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "byte end range: "+strconv.Itoa(endRange))
		} else {

			segURL = http.GetNextSegment(mpdList[mpdListIndex], segmentNumber, repRate, mimeTypes[mimeTypeIndex])
		}
		logging.DebugPrint(glob.DebugFile, debugLog, "DEBUG: ", "current segment URL: "+segURL)

		// Collaborative Code - Start
		OriginalURL := currentURL
		OriginalBaseURL := baseURL
		baseJoined := baseURL + segURL
		urlHeaderString := http.JoinURL(currentURL, baseURL+segURL, debugLog)
		if Noden.ClientName != glob.CollabPrintOff && Noden.ClientName != "" {
			currentURL = Noden.Search(urlHeaderString, segmentDuration, true, profile)

			logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "current URL joined: "+currentURL)
			currentURL = strings.Split(currentURL, "::")[0]
			logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "current URL joined: "+currentURL)
			urlSplit := strings.Split(currentURL, "/")
			logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "current URL joined: "+urlSplit[len(urlSplit)-1])
			baseJoined = urlSplit[len(urlSplit)-1]
		}
		// Collaborative Code - End

		// Start Time of this segment
		currentTime := time.Now()

		// Download the segment - add the segment duration to the file name
		switch adapt {
		case glob.ConventionalAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
		case glob.ElasticAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
		case glob.ProgressiveAlg:
			rtt, segSize = http.GetFileProgressively(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, debugLog, AudioByteRange, profile)
		case glob.LogisticAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
		case glob.MeanAverageAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
		case glob.GeomAverageAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
		case glob.EMWAAverageAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
		case glob.TestAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
		case glob.ArbiterAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
		case glob.BBAAlg:
			rtt, segSize, protocol, segmentFileName, P1203Header = http.GetFile(currentURL, baseJoined, fileDownloadLocation, isByteRangeMPD, startRange, endRange, segmentNumber, segmentDuration, true, quicBool, glob.DebugFile, debugLog, useTestbedBool, repRate, saveFilesBool, AudioByteRange, profile)
		}

		// arrival and delivery times for this segment
		arrivalTime = int(time.Since(startTime).Nanoseconds() / (glob.Conversion1000 * glob.Conversion1000))
		deliveryTime := int(time.Since(currentTime).Nanoseconds() / (glob.Conversion1000 * glob.Conversion1000)) //Time in milliseconds
		thisRunTimeVal := int(time.Since(nextRunTime).Nanoseconds() / (glob.Conversion1000 * glob.Conversion1000))

		nextRunTime = time.Now()

		// some times we want to wait for an initial number of segments before stream begins
		// no need to do asny printouts when we are replacing this chunk
		// && !hlsReplaced
		if initBuffer <= waitToPlayCounter {

			// get the segment less the initial buffer
			// this needs to be based on running time and not based on number segments
			// I'll need a function for this
			//playoutSegmentNumber := segmentNumber - initBuffer

			// only print this out if we are not hls replaced
			if !hlsUsed {
				// print out the content of the segment that is currently passed to the player
				var printLogs []map[int]logging.SegPrintLogInformation
				printLogs = append(printLogs, mapSegmentLogPrintout)
				logging.PrintPlayOutLog(arrivalTime, initBuffer, printLogs, glob.LogDownload, printLog, printHeadersData)
			}

			// get the current buffer (excluding the current segment)
			currentBuffer := (bufferLevel - thisRunTimeVal)

			// if we have a buffer level then we have no stalls
			if currentBuffer >= 0 {
				stallTime = 0

				// if the buffer is empty, then we need to calculate
			} else {
				stallTime = currentBuffer
			}

			// To have the bufferLevel we take the max between the remaining buffer and 0, we add the duration of the segment we downloaded
			bufferLevel = utils.Max(bufferLevel-thisRunTimeVal, 0) + (segmentDuration * glob.Conversion1000)

			// increment the waitToPlayCounter
			waitToPlayCounter++

		} else {
			// add to the current buffer before we start to play
			bufferLevel += (segmentDuration * glob.Conversion1000)
			// increment the waitToPlayCounter
			waitToPlayCounter++
		}

		// check if the buffer level is higher than the max buffer
		if bufferLevel > maxBuffer*glob.Conversion1000 {
			// retrieve the time it is going to sleep from the buffer level
			// sleep until the max buffer level is reached
			sleepTime := bufferLevel - (maxBuffer * glob.Conversion1000)
			// sleep
			time.Sleep(time.Duration(sleepTime) * time.Millisecond)

			// reset the buffer to the new value less sleep time - should equal maxBuffer
			bufferLevel -= sleepTime
		}

		// some times we want to wait for an initial number of segments before stream begins
		// if we are going to print out some additonal log headers, then get these values
		if extendPrintLog && initBuffer < waitToPlayCounter {
			// base the play out position on the buffer level
			playPosition = segmentDurationTotal + (segmentDuration * glob.Conversion1000) - bufferLevel
			// we need to keep a tab on the different size segments - use this for now
			segmentDurationTotal += (segmentDuration * glob.Conversion1000)
		} else {
			segmentDurationTotal += (segmentDuration * glob.Conversion1000)
		}

		// if we are going to print out some additonal log headers, then get these values
		if extendPrintLog {

			// get the current codec
			repCodec = mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].Codecs

			// change the codec into something we can understand
			// switch {
			// case strings.Contains(repCodec, "avc"):
			// 	// set the inital rep_rate to the lowest value
			// 	repCodec = glob.RepRateCodecAVC
			// case strings.Contains(repCodec, "hev"):
			// 	repCodec = glob.RepRateCodecHEVC
			// case strings.Contains(repCodec, "vp"):
			// 	repCodec = glob.RepRateCodecVP9
			// case strings.Contains(repCodec, "av1"):
			// 	repCodec = glob.RepRateCodecAV1
			// }

			switch {
			case strings.Contains(repCodec, "avc"):
				repCodec = glob.RepRateCodecAVC
			case strings.Contains(repCodec, "hev"):
				repCodec = glob.RepRateCodecHEVC
			case strings.Contains(repCodec, "hvc1"):
				repCodec = glob.RepRateCodecHEVC
			case strings.Contains(repCodec, "vp"):
				repCodec = glob.RepRateCodecVP9
			case strings.Contains(repCodec, "av1"):
				repCodec = glob.RepRateCodecAV1
			case strings.Contains(repCodec, "mp4a"):
				repCodec = glob.RepRateCodecAudio
			case strings.Contains(repCodec, "ac-3"):
				repCodec = glob.RepRateCodecAudio
			}

			// get rep_rate height, width and frames per second
			repHeight = mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].Height
			repWidth = mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].Width
			repFps = mpdList[mpdListIndex].Periods[0].AdaptationSet[mimeTypes[mimeTypeIndex]].Representation[repRate].FrameRate
		}

		// calculate the throughtput (we get the segSize while downloading the file)
		// multiple segSize by 8 to get bits and not bytes
		thr := algo.CalculateThroughtput(segSize*8, deliveryTime)

		// save the bitrate from the input segment (less the header info)
		var kbps float64
		if getQoEBool {
			if val, ok := printHeadersData[glob.P1203Header]; ok {
				if val == "on" || val == "On" {

					// we use this to read from a file
					// kbps = qoe.GetKBPS(segmentFileName, int64(segmentDuration), debugLog, isByteRangeMPD, segSize)

					// we do this to read from our buffer values
					kbps = P1203Header
				}
			}
			// lets move the logic setup for the QoE values from the algorithms to player
			// we don't need to save the segRate as this is also called 'Bandwidth'
			// segRate := float64(log[j].Bandwidth)

			// add this to the seg rate slice
			if segmentNumber > 1 {
				// append to the segRates list
				segRates = append(mapSegmentLogPrintout[segmentNumber-1].SegmentRates, float64(bandwithList[repRate]))
				// sum the seg rates
				sumSegRate = mapSegmentLogPrintout[segmentNumber-1].SumSegRate + float64(bandwithList[repRate])
				// sum the total stall duration
				totalStallDur = float64(mapSegmentLogPrintout[segmentNumber-1].StallTime) + float64(stallTime)
				// get the number of stalls
				if stallTime > 0 {
					// increment the number of stalls
					nStalls = mapSegmentLogPrintout[segmentNumber-1].NumStalls + 1
				} else {
					// otherwise save the number of stalls from the previous log
					nStalls = mapSegmentLogPrintout[segmentNumber-1].NumStalls
				}
				// get the number of switches
				if bandwithList[repRate] == mapSegmentLogPrintout[segmentNumber-1].Bandwidth {
					// store the previous value of switches
					nSwitches = mapSegmentLogPrintout[segmentNumber-1].NumSwitches
				} else {
					// increment the number of switches
					nSwitches = mapSegmentLogPrintout[segmentNumber-1].NumSwitches + 1
				}
				rateDifference = math.Abs(float64(bandwithList[repRate]) - float64(mapSegmentLogPrintout[segmentNumber-1].Bandwidth))
				sumRateChange = mapSegmentLogPrintout[segmentNumber-1].SumRateChange + rateDifference
				rateChange = append(mapSegmentLogPrintout[segmentNumber-1].RateChange, rateDifference)

			} else {

				// otherwise create the list
				segRates = append(segRates, float64(bandwithList[repRate]))
				// sum the seg rates
				sumSegRate = float64(bandwithList[repRate])
				// sum the total stall duration
				totalStallDur = float64(stallTime)
				// get the number of stalls
				if stallTime > 0 {
					// increment the number of stalls
					nStalls = 1
				} else {
					// otherwise set to zero (may not be needed, go might default to zero)
					nStalls = 0
				}
				// get the number of switches
				nSwitches = 0
			}
		}

		// Print to output log
		//printLog(strconv.Itoa(segmentNumber), strconv.Itoa(arrivalTime), strconv.Itoa(deliveryTime), strconv.Itoa(Abs(stallTime)), strconv.Itoa(bandwithList[repRate]/1000), strconv.Itoa((segSize*8)/deliveryTime), strconv.Itoa((segSize*8)/(segmentDuration*1000)), strconv.Itoa(segSize), strconv.Itoa(bufferLevel), adapt, strconv.Itoa(segmentDuration*1000), extendPrintLog, repCodec, strconv.Itoa(repWidth), strconv.Itoa(repHeight), strconv.Itoa(repFps), strconv.Itoa(playPosition), strconv.FormatFloat(float64(rtt.Nanoseconds())/1000000, 'f', 3, 64), fileDownloadLocation)

		// store the current segment log output information in a map
		printInformation := logging.SegPrintLogInformation{
			ArrivalTime:          arrivalTime,
			DeliveryTime:         deliveryTime,
			StallTime:            stallTime,
			Bandwidth:            bandwithList[repRate],
			DelRate:              thr,
			ActRate:              (segSize * 8) / (segmentDuration * glob.Conversion1000),
			SegSize:              segSize,
			P1203HeaderSize:      P1203Header,
			BufferLevel:          bufferLevel,
			Adapt:                adapt,
			SegmentDuration:      segmentDuration,
			ExtendPrintLog:       extendPrintLog,
			RepCodec:             repCodec,
			RepWidth:             repWidth,
			RepHeight:            repHeight,
			RepFps:               repFps,
			PlayStartPosition:    segmentDurationTotal,
			PlaybackTime:         playPosition,
			Rtt:                  float64(rtt.Nanoseconds()) / (glob.Conversion1000 * glob.Conversion1000),
			FileDownloadLocation: fileDownloadLocation,
			RepIndex:             repRate,
			MpdIndex:             mpdListIndex,
			AdaptIndex:           mimeTypes[mimeTypeIndex],
			SegmentIndex:         nextSegmentNumber,
			SegReplace:           hlsReplaced,
			Played:               false,
			HTTPprotocol:         protocol,
			P1203Kbps:            kbps,
			SegmentFileName:      segmentFileName,
			SegmentRates:         segRates,
			SumSegRate:           sumSegRate,
			TotalStallDur:        totalStallDur,
			NumStalls:            nStalls,
			NumSwitches:          nSwitches,
			RateDifference:       rateDifference,
			SumRateChange:        sumRateChange,
			RateChange:           rateChange,
			MimeType:             mimeType,
			Profile:              profile,
		}

		// this saves per segment number so from 1 on, and not 0 on
		// remember this :)
		mapSegmentLogPrintout[incrementalSegmentNumber] = printInformation

		// if we want to create QoE, then pass in the printInformation and save the QoE values to log
		// don't save json when using collaborative
		var saveCollabFilesBool bool
		if Noden.ClientName != glob.CollabPrintOff && Noden.ClientName != "" {
			saveCollabFilesBool = false
		} else {
			saveCollabFilesBool = saveFilesBool
		}
		if getQoEBool {
			qoe.CreateQoE(&mapSegmentLogPrintout, debugLog, initBuffer, bandwithList[highestMPDrepRateIndex], printHeadersData, saveCollabFilesBool, audioRate, audioCodec)
		}

		// to calculate throughtput and select the repRate from it (in algorithm.go)
		switch adapt {
		//Conventional Algo
		case glob.ConventionalAlg:
			algo.Conventional(&thrList, thr, &repRate, bandwithList, lowestMPDrepRateIndex)
			//Harmonic Mean Algo
		case glob.ElasticAlg:
			algo.ElasticAlgo(&thrList, thr, deliveryTime, maxBuffer, &repRate, bandwithList, &staticAlgParameter, bufferLevel, kP, kI, lowestMPDrepRateIndex)
		//Progressive Algo
		case glob.ProgressiveAlg:
			algo.Conventional(&thrList, thr, &repRate, bandwithList, lowestMPDrepRateIndex)
		//Logistic Algo
		case glob.LogisticAlg:
			algo.Logistic(&thrList, thr, &repRate, bandwithList, bufferLevel,
				highestMPDrepRateIndex, lowestMPDrepRateIndex, glob.DebugFile, debugLog,
				maxBufferLevel)
			logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "reprate returned: "+strconv.Itoa(repRate))
		//Mean Average Algo
		case glob.MeanAverageAlg:
			algo.MeanAverageAlgo(&thrList, thr, &repRate, bandwithList, lowestMPDrepRateIndex)
		//Geometric Average Algo
		case glob.GeomAverageAlg:
			algo.GeomAverageAlgo(&thrList, thr, &repRate, bandwithList, lowestMPDrepRateIndex)
		//Exponential Average Algo
		case glob.EMWAAverageAlg:
			algo.EMWAAverageAlgo(&thrList, &repRate, exponentialRatio, 3, thr, bandwithList, lowestMPDrepRateIndex)

		case glob.ArbiterAlg:

			repRate = algo.CalculateSelectedIndexArbiter(thr, segmentDuration*1000, segmentNumber, maxBufferLevel,
				repRate, &thrList, streamDuration, mpdList[mpdListIndex], currentURL,
				mimeTypes[mimeTypeIndex], segmentNumber, baseURL, debugLog, deliveryTime, bufferLevel,
				highestMPDrepRateIndex, lowestMPDrepRateIndex, bandwithList,
				segSize)
		case glob.BBAAlg:

			repRate = algo.CalculateSelectedIndexBba(thr, segmentDuration*1000, segmentNumber, maxBufferLevel,
				repRate, &thrList, streamDuration, mpdList[mpdListIndex], currentURL,
				mimeTypes[mimeTypeIndex], segmentNumber, baseURL, debugLog, deliveryTime, bufferLevel,
				highestMPDrepRateIndex, lowestMPDrepRateIndex, bandwithList)

		case glob.TestAlg:
		}

		logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", adapt+" has choosen rep_Rate "+strconv.Itoa(repRate)+" @ a rate of "+strconv.Itoa(bandwithList[repRate]/glob.Conversion1000))

		//Increase the segment number
		segmentNumber++
		incrementalSegmentNumber++

		// break out if we have downloaded all of our segments
		if segmentDurationTotal+(segmentDuration*glob.Conversion1000) > streamDuration {
			logging.DebugPrint(glob.DebugFile, debugLog, "\nDEBUG: ", "We have downloaded all segments at the end of the streamLoop - segment total: "+strconv.Itoa(segmentDurationTotal)+"  current segment duration: "+strconv.Itoa(segmentDuration*glob.Conversion1000)+" gives a total of:  "+strconv.Itoa(segmentDurationTotal+(segmentDuration*glob.Conversion1000)))

			if mimeTypeIndex == len(mimeTypes)-1 {
				// save the current log
				streamStructs[mimeTypeIndex].MapSegmentLogPrintout = mapSegmentLogPrintout
				// get the logs for all adaptationSets
				for thisMimeTypeIndex := range mimeTypes {
					mapSegmentLogPrintouts = append(mapSegmentLogPrintouts, streamStructs[thisMimeTypeIndex].MapSegmentLogPrintout)
				}
				return segmentNumber, mapSegmentLogPrintouts
			}
		}

		// save info for the next segment
		streaminfo := http.StreamStruct{
			SegmentNumber:         incrementalSegmentNumber,
			CurrentURL:            OriginalURL,
			InitBuffer:            initBuffer,
			MaxBuffer:             maxBuffer,
			CodecName:             codecName,
			Codec:                 codec,
			UrlString:             urlString,
			UrlInput:              urlInput,
			MpdList:               mpdList,
			Adapt:                 adapt,
			MaxHeight:             maxHeight,
			IsByteRangeMPD:        isByteRangeMPD,
			StartTime:             startTime,
			NextRunTime:           nextRunTime,
			ArrivalTime:           arrivalTime,
			OldMPDIndex:           oldMPDIndex,
			NextSegmentNumber:     nextSegmentNumber,
			Hls:                   hls,
			HlsBool:               hlsBool,
			MapSegmentLogPrintout: mapSegmentLogPrintout,
			StreamDuration:        streamDuration,
			ExtendPrintLog:        extendPrintLog,
			HlsUsed:               hlsUsed,
			BufferLevel:           bufferLevel,
			SegmentDurationTotal:  segmentDurationTotal,
			Quic:                  quic,
			QuicBool:              quicBool,
			BaseURL:               OriginalBaseURL,
			DebugLog:              debugLog,
			AudioContent:          audioContent,
			RepRate:               repRate,
			BandwithList:          bandwithList,
			Profile:               profile,
			LowestMPDrepRateIndex: lowestMPDrepRateIndex,
			WaitToPlayCounter:     waitToPlayCounter,
		}
		streamStructs[mimeTypeIndex] = streaminfo

		// this gets the index for the next MPD and the segment number for the next chunk
		stopPlayer = false

		// get some new info - update only on the audio adaptationset
		// or if there is only one adaptationset
		if len(mimeTypes) == 1 || (mimeType == glob.RepRateCodecAudio && len(mimeTypes)-1 > 0) {
			// set the next segment duration
			stopPlayer, oldMPDIndex, nextSegmentNumber = http.GetNextSegmentDuration(segmentDurationArray, segmentDuration*glob.Conversion1000, segmentDurationTotal, glob.DebugFile, streamStructs[mimeTypeIndex].DebugLog, segmentDurationArray[mpdListIndex], streamStructs[mimeTypeIndex].StreamDuration)
			for mimeTypeIndex := range mimeTypes {
				streamStructs[mimeTypeIndex].OldMPDIndex = oldMPDIndex
				streamStructs[mimeTypeIndex].NextSegmentNumber = nextSegmentNumber
				// mapSegmentLogPrintouts[mimeTypeIndex] = streamStructs[mimeTypeIndex].MapSegmentLogPrintout
				mapSegmentLogPrintouts = append(mapSegmentLogPrintouts, streamStructs[mimeTypeIndex].MapSegmentLogPrintout)
			}
		}
	}

	// this gets the index for the next MPD and the segment number for the next chunk
	// stopPlayer := false
	//
	// // get some new info
	// for mimeTypeIndex := range mimeTypes {
	// 	stopPlayer, oldMPDIndex, nextSegmentNumber = http.GetNextSegmentDuration(segmentDurationArray, segmentDuration*glob.Conversion1000, segmentDurationTotal, glob.DebugFile, streamStructs[mimeTypeIndex].DebugLog, segmentDurationArray[mpdListIndex], streamStructs[mimeTypeIndex].StreamDuration)
	// 	streamStructs[mimeTypeIndex].OldMPDIndex = oldMPDIndex
	// 	streamStructs[mimeTypeIndex].NextSegmentNumber = nextSegmentNumber
	// 	// mapSegmentLogPrintouts[mimeTypeIndex] = streamStructs[mimeTypeIndex].MapSegmentLogPrintout
	// 	mapSegmentLogPrintouts = append(mapSegmentLogPrintouts, streamStructs[mimeTypeIndex].MapSegmentLogPrintout)
	// }

	// stream the next chunk
	if !stopPlayer {
		segmentNumber, mapSegmentLogPrintouts = streamLoop(streamStructs, Noden)
	}

	return segmentNumber, mapSegmentLogPrintouts

}
