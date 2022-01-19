package main

import (
	"fmt"
	"log"
	"strings"
	"time"
)

//ReportResultJSON is a struct convert from PingResult to desire json output struct
type ReportResultJSON struct {
	Hostname            string `json:"hostname"`
	Submitted           int    `json:"submitted"`
	Confirmed           int    `json:"confirmed"`
	Loss                string `json:"loss"`
	ConfirmationMessage string `json:"confirmation"`
	TimeStamp           string `json:"ts"`
	Error               string `json:"error"`
}

//DataPoint1MinJson is a struct return to api
type DataPoint1MinJson struct {
	NumOfDataPoint int                       `json:"num_data_point"`
	NumOfNoData    int                       `json:"num_of_nodata"`
	Data           []DataPoint1MinResultJSON `json:"data"`
}

//ReportPingResultJSON is a struct convert from PingResult to desire json output struct
type DataPoint1MinResultJSON struct {
	Submitted int    `json:"submitted"`
	Confirmed int    `json:"confirmed"`
	Loss      string `json:"loss"`
	TimeStamp string `json:"ts"`
	Error     string `json:"error"`
}

//SlackText slack structure
type SlackText struct {
	SText string `json:"text"`
	SType string `json:"type"`
}

//Block slack structure
type Block struct {
	BlockType string    `json:"type"`
	BlockText SlackText `json:"text"`
}

//SlackPayload slack structure
type SlackPayload struct {
	Blocks []Block `json:"blocks"`
}

//statisticResult for caucualate report
type statisticResult struct {
	NumOfRecords int
	Submiited    float64
	Confirmed    float64
	Loss         float64
	Count        int
	ErrCount     int
}

//ToReportJoson convert PingResult to Json Format
func ToReportJoson(r *PingResult) ReportResultJSON {
	// Check result
	jsonResult := ReportResultJSON{Hostname: r.Hostname, Submitted: r.Submitted, Confirmed: r.Confirmed,
		ConfirmationMessage: r.ConfirmationMessage, Error: r.Error}
	loss := fmt.Sprintf("%3.1f%s", r.Loss, "%")
	jsonResult.Loss = loss
	ts := time.Unix(r.TimeStamp, 0)
	jsonResult.TimeStamp = ts.Format(time.RFC3339)
	return jsonResult
}

func To1MinWindowJson(r *PingResult) DataPoint1MinResultJSON {
	// Check result
	jsonResult := DataPoint1MinResultJSON{Submitted: r.Submitted, Confirmed: r.Confirmed, Error: r.Error}
	loss := fmt.Sprintf("%3.1f%s", r.Loss, "%")
	jsonResult.Loss = loss
	ts := time.Unix(r.TimeStamp, 0)
	jsonResult.TimeStamp = ts.Format(time.RFC3339)
	return jsonResult
}

//ToPayload get the report within specified minutes
func (s *SlackPayload) ToPayload(c Cluster, data []PingResult, stats statisticResult) {
	records, err := reportBody(data, stats)
	description := "( Submitted, Confirmed, Loss, min/mean/max/stddev ms )"
	body := Block{}
	if err == nil {
		header := Block{
			BlockType: "section",
			BlockText: SlackText{
				SType: "mrkdwn",
				SText: fmt.Sprintf("%d results availible in %s. %s\n",
					stats.Count, c, fmt.Sprintf("Average Loss %3.1f%s", stats.Loss, "%")),
			},
		}
		s.Blocks = append(s.Blocks, header)

		body = Block{
			BlockType: "section",
			BlockText: SlackText{SType: "mrkdwn", SText: fmt.Sprintf("```%s\n%s```", description, records)},
		}
		s.Blocks = append(s.Blocks, body)
	}
}

func generateReportData(pr []PingResult) statisticResult {
	var sumSub, sumConf, sumLoss float64
	count := 0
	errCount := 0
	for _, e := range pr {
		sumSub += float64(e.Submitted)
		sumConf += float64(e.Confirmed)
		sumLoss += e.Loss
		count++
	}
	avgLoss := sumLoss / float64(count)
	avgSub := sumSub / float64(count)
	avgConf := sumConf / float64(count)
	return statisticResult{NumOfRecords: len(pr), Submiited: avgSub, Confirmed: avgConf, Loss: avgLoss, Count: count, ErrCount: errCount}
}

func reportBody(pr []PingResult, st statisticResult) (string, error) {
	if st.Count <= 0 {
		return "()", NoPingResultRecord
	}

	text := ""
	for _, e := range pr {
		cmsg := strings.Split(e.ConfirmationMessage, " ")
		var confdata string
		if len(cmsg) < 4 {
			log.Println("split confirmationMessage error:", cmsg, " PingResult=>", e)
			confdata = e.ConfirmationMessage
		} else {
			confdata = cmsg[2]
		}
		text = fmt.Sprintf("%s( %d, %d, %3.1f, %s )\n", text, e.Submitted, e.Confirmed, e.Loss, confdata)
	}

	return text, nil
}

func generateDataPoint1Min(startTime int64, endTime int64, pr []PingResult) ([]DataPoint1MinResultJSON, int) {
	window := int64(60)
	datapoint1MinRet := []PingResult{}
	nodata := 0
	for periodend := endTime; periodend > startTime; periodend = periodend - window {
		count := 0
		windowResult := PingResult{}
		for _, result := range pr {
			if result.TimeStamp <= periodend && result.TimeStamp > periodend-window {
				windowResult.Submitted = windowResult.Submitted + result.Submitted
				windowResult.Confirmed = windowResult.Confirmed + result.Confirmed
				windowResult.TimeStamp = periodend // use the newest one for easier tracking
				windowResult.Hostname = result.Hostname
				count++
			}
		}
		if count == 0 {
			windowResult.Error = "No Data"
			windowResult.Submitted = 0
			windowResult.Confirmed = 0
			windowResult.Loss = 0
			windowResult.TimeStamp = periodend
			nodata++
		} else {
			if windowResult.Submitted > 0 {
				windowResult.Loss = (float64(windowResult.Submitted-windowResult.Confirmed) / float64(windowResult.Submitted)) * 100
			}
		}

		windowResult.PingType = string(DataPoint1Min)
		datapoint1MinRet = append(datapoint1MinRet, windowResult)

		if windowResult.Loss > 0 {
			log.Println(windowResult)
		}
	}
	ret := []DataPoint1MinResultJSON{}
	for _, e := range datapoint1MinRet {
		ret = append(ret, To1MinWindowJson(&e))
		if e.Loss > 0 {
			log.Println("Loss:", e.Loss, "ts:", e.TimeStamp, " date:", ret[len(ret)-1].TimeStamp)

		}
	}
	return ret, nodata
}