package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"banking-circle-payment-systems/DevelopmentExcellence/_git/loki-exporter/pkg/config"
	"banking-circle-payment-systems/DevelopmentExcellence/_git/loki-exporter/pkg/helpers"
	"banking-circle-payment-systems/DevelopmentExcellence/_git/loki-exporter/pkg/logger"

	"github.com/go-co-op/gocron/v2"
)

var (
	dirlogPaths      string
	checkPoint       string
	nameSpace        string
	lokiAddr         string
	lokiChunkSize    time.Duration
	scheduleInterval int
	debugFlag        bool
	maxQueryLogs     int
)

func task(log logger.Logger) {
	l := helpers.New(
		lokiAddr+"/loki/api/v1/query_range",
		checkPoint,
		nameSpace,
		log,
		maxQueryLogs,
	)

	path := dirlogPaths + GetDirectoryName()
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			log.Error(err.Error())
		}
	}

	filename := GetFilenameDate()

	filename = path + "/" + filename

	if err := l.FetchAndProcessLogs(lokiChunkSize, filename); err != nil {
		fmt.Printf("failed to fetch logs: %v \n", err)
	}

	log.Info("Exporting logs from all iterations at " + time.Now().String())
}

func main() {
	cfg := config.Load()

	flag.StringVar(&lokiAddr, "addr", "", "supply the loki address")
	flag.StringVar(&nameSpace, "ns", "", "Namespace Qurey params")
	flag.StringVar(&dirlogPaths, "logpath", "", "some log path to use /path/folder1/")
	flag.StringVar(
		&checkPoint,
		"checkpoint",
		"",
		"some checkpoint file path to use /path/folder1/checkpoint.json",
	)
	flag.IntVar(&scheduleInterval, "scheduleint", 0, "Schedule interval for the cron job")
	flag.DurationVar(&lokiChunkSize, "chunk", 0, "Chunk size for the job")
	flag.BoolVar(&debugFlag, "debug", false, "The debug flag")
	flag.IntVar(&maxQueryLogs, "maxquerylogs", 0, "Max logs for the query to loki")

	flag.Parse()
	if lokiAddr == "" {
		lokiAddr = cfg.LokiAddr
	}
	if nameSpace == "" {
		nameSpace = cfg.NamespaceQuery
	}
	if dirlogPaths == "" {
		dirlogPaths = cfg.DirLogPaths
	}
	if scheduleInterval == 0 {
		scheduleInterval = cfg.ScheduleInterval
	}
	if lokiChunkSize == 0 {
		lokiChunkSize = time.Duration(cfg.LokiChunkSize) * time.Minute
	}
	if checkPoint == "" {
		checkPoint = cfg.Checkpoint
	}
	if !debugFlag {
		debugFlag = cfg.DebugFlag
	}
	if maxQueryLogs == 0 {
		maxQueryLogs = cfg.MaxQueryLogs
	}

	log := logger.New(debugFlag)
	log.Info("exporter is running with the following parameters: ")
	log.Info(fmt.Sprintf("Namespace Query: %s", nameSpace))
	log.Info(fmt.Sprintf("Dirictory path for logs: %s", dirlogPaths))
	log.Info(fmt.Sprintf("Schedule interval in minutes: %v", scheduleInterval))
	log.Info(fmt.Sprintf("The chunk slized interval in minutes: %v", lokiChunkSize))
	log.Info(fmt.Sprintf("The checkpoint file: %s", checkPoint))
	log.Info(fmt.Sprintf("Debug flag: %v", debugFlag))
	log.Info(fmt.Sprintf("The max paginated logs to query: %d", maxQueryLogs))

	s, err := gocron.NewScheduler()
	if err != nil {
		log.Error(fmt.Sprintf("Gocron error: %v", err))
		return
	}
	_, err = s.NewJob(
		gocron.DurationJob(
			time.Duration(scheduleInterval)*time.Minute,
		),
		gocron.NewTask(func() {
			task(log)
		}),
	)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to create job: %v", err))
		return
	}
	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		s.Shutdown()
	}()

	s.Start()
	select {}
}

func GetFilenameDate() string {
	// Use layout string for time format.
	const layout = "01-02-2006"
	// Place now in the string.
	t := time.Now()
	return "log-" + t.Format(layout) + ".txt"
}

func GetDirectoryName() string {
	// Use layout string for time format.
	const layout = "01-2006"
	t := time.Now()
	return t.Format(layout)
}
