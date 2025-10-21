package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dkr290/go-loki-exporter/pkg/logger"
)

// LokiResponse represents the response from Loki's query API

type LokiResponse struct {
	Data struct {
		Result []struct {
			Stream map[string]string `json:"stream"`
			Values [][]string        `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

// Checkpoint stores the last fetched timestamp
type Checkpoint struct {
	LastTimestamp time.Time `json:"lastTimestamp"`
}

// the config for the url the query and the checkpoint filename
type LokiConfig struct {
	LokiURL        string
	CheckpointFile string
	Query          string
	Log            logger.Logger
	MaxQueryLogs   int
}

// initialize function
func New(url, checkPointFile, query string, log logger.Logger, maxqueryLogs int) *LokiConfig {
	return &LokiConfig{
		LokiURL:        url,
		CheckpointFile: checkPointFile,
		Query:          query,
		Log:            log,
		MaxQueryLogs:   maxqueryLogs,
	}
}

var httpClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
	},
}

// FetchAndProcessLogs fetches logs from Loki and processes them - doing all the work
func (c *LokiConfig) FetchAndProcessLogs(
	chunkTime time.Duration,
	filename string,
) error {
	// Load the checkpoint
	var checkpoint *Checkpoint
	var err error
	count := 0
	for {
		if count < 3 {
			checkpoint, err = c.loadCheckpoint()
			if err != nil {
				c.Log.Error(fmt.Sprintf("failed to load checkpoint: %v", err))
				count++
			} else {
				break
			}
		} else {
			err := os.Remove(c.CheckpointFile)
			if err != nil {
				return fmt.Errorf("error deleting file: %v", err)
			}
			count = 0
		}
	}

	// Calculate the start and end time for the query
	// Add a buffer to endTime to account for log ingestion delays.

	buffer := 10 * time.Second
	startTime := checkpoint.LastTimestamp
	currentTime := startTime.Add(-buffer)
	endTime := time.Now()

	for currentTime.Before(endTime) {
		chunkEndTime := currentTime.Add(chunkTime)
		c.Log.Info(strings.Repeat("=", 10) + "start chunk query")
		// if it is after or the same time just be the same which it time now if not it is chunkTime
		if chunkEndTime.After(endTime) {
			chunkEndTime = endTime
		}
		// Fetch logs from Loki
		c.Log.Info(fmt.Sprintf(
			"Querying Loki for chunk: %s to %s",
			currentTime,
			chunkEndTime,
		))
		cursor := currentTime
		for {
			logs, lastTimestamp, err := c.fetchLogsFromLoki(
				currentTime,
				chunkEndTime,
				cursor,
				c.MaxQueryLogs,
			)
			if err != nil {
				return fmt.Errorf(
					"failed to fetch logs from %v to %v: %v",
					cursor,
					chunkEndTime,
					err,
				)
			}
			WriteLogs(filename, logs)
			c.Log.Debug(fmt.Sprintf("Saving logs from pagination size %d", c.MaxQueryLogs))
			if len(logs) < c.MaxQueryLogs || lastTimestamp.IsZero() ||
				!lastTimestamp.Before(chunkEndTime) {
				break
			}
			cursor = lastTimestamp.Add(time.Nanosecond)

		}
		// Extract timestamps from logs and find the latest

		checkpoint.LastTimestamp = chunkEndTime
		err = c.saveCheckpoint(checkpoint)
		if err != nil {
			return fmt.Errorf("failed to save checkpoint: %v", err)
		}
		c.Log.Info(fmt.Sprintf("Save the chunk last timestamp %v", chunkEndTime))
		currentTime = chunkEndTime // move to the next chunk
		c.Log.Info(strings.Repeat("=", 10) + "End iteration Loop")
		time.Sleep(2 * time.Second)

	}
	c.Log.Info(fmt.Sprintf("Savinglogs last timestamp of checkpoint %v", currentTime))

	return nil
}

// fetchLogsFromLoki fetches logs from Loki within the specified time range
// TODO to verify the query here
func (c *LokiConfig) fetchLogsFromLoki(
	startTime, endTime time.Time,
	cursor time.Time, maxqueryLogs int,
) ([]string, time.Time, error) {
	// query := `{job="your-job-name"}` // Replace with your Loki query
	c.Log.Debug(fmt.Sprintf("Queriyng with limit %s", strconv.Itoa(maxqueryLogs)))

	queryParams := url.Values{}
	queryParams.Set("query", c.Query)
	effectiveStart := startTime
	if !cursor.IsZero() {
		effectiveStart = cursor
	}

	queryParams.Set("start", fmt.Sprintf("%v", effectiveStart.UnixNano()))
	queryParams.Set("end", fmt.Sprintf("%v", endTime.UnixNano()))
	queryParams.Set("limit", strconv.Itoa(maxqueryLogs))
	queryParams.Set("direction", "forward")
	fullURL := fmt.Sprintf("%s?%s", c.LokiURL, queryParams.Encode())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to create HTTP request : %v", err)
	}
	// response and request with get
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to make HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// read all response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to read response body: %v", err)
	}
	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, time.Time{}, fmt.Errorf(
			"loki API returned unexpected status code %d: %s",
			resp.StatusCode,
			string(body),
		)
	}
	// // Debug: Print raw response
	// fmt.Println("Raw Response Body:", string(body))

	var lokiResponse LokiResponse
	err = json.Unmarshal(body, &lokiResponse)
	if err != nil {
		rawLog := fmt.Sprintf("[ERROR: Failed to parse Loki response] %s", string(body))
		return []string{rawLog}, time.Time{}, nil // Return raw data as a log line
	}
	// the sorting function of logs by timestamp
	// without it it appeared a bit mixed in closed timestamps not chronologically design
	logs := sortingLogs(&lokiResponse)
	var lastTimestamp time.Time
	for i := len(lokiResponse.Data.Result) - 1; i >= 0; i-- {
		values := lokiResponse.Data.Result[i].Values
		if len(values) > 0 {
			tsNano, _ := strconv.ParseInt(values[len(values)-1][0], 10, 64)
			lastTimestamp = time.Unix(0, tsNano)
			break
		}
	}
	return logs, lastTimestamp, nil
}

// loadCheckpoint loads the checkpoint from a file
func (c *LokiConfig) loadCheckpoint() (*Checkpoint, error) {
	if _, err := os.Stat(c.CheckpointFile); os.IsNotExist(err) {
		// If the checkpoint file doesn't exist, start from now minus 35 minutes
		c.Log.Warn("Checkpoint does not exists, loading last 35 min")
		return &Checkpoint{LastTimestamp: time.Now().Add(-35 * time.Minute)}, nil
	}

	data, err := os.ReadFile(c.CheckpointFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read checkpoint file: %v", err)
	}

	var checkpoint Checkpoint
	err = json.Unmarshal(data, &checkpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal checkpoint: %v", err)
	}

	return &checkpoint, nil
}

// saveCheckpoint saves the checkpoint to a file
func (c *LokiConfig) saveCheckpoint(checkpoint *Checkpoint) error {
	data, err := json.Marshal(checkpoint)
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %v", err)
	}

	err = os.WriteFile(c.CheckpointFile, data, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write checkpoint file: %v", err)
	}

	return nil
}
