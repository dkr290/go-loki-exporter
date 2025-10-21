package helpers

import (
	"bufio"
	"fmt"
	"log"
	"os"
)

func WriteLogs(filename string, logs []string) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()
	// Process and write logs to file
	for _, log := range logs {
		// fmt.Println("Log:", log)                 // Print log to console
		_, err := writer.WriteString(log + "\n") // Write log to file
		if err != nil {
			fmt.Printf("Error writing to file: %v\n", err)
			return
		}
	}
	writer.Flush()
}
