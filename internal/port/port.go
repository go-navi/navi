// Package port provides utilities for TCP port checking and management
package port

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/go-navi/navi/internal/logger"
	"github.com/go-navi/navi/internal/utils"
)

// VerifyPortAvailability checks if a TCP port is accessible within the specified timeout
func VerifyPortAvailability(portNumber int, timeoutSeconds float64, getContextPrefix func() string) error {
	// Default timeout of 30 seconds if not specified
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}

	portAddress := ":" + strconv.Itoa(portNumber)
	endTime := time.Now().Add(time.Duration(int(timeoutSeconds*1000)) * time.Millisecond)
	prefix := getContextPrefix()

	// Initial log message
	logRemainingTime(prefix, portNumber, endTime)
	lastLogTime := time.Now()

	// Poll until timeout
	for time.Now().Before(endTime) {
		// Try connection
		if conn, err := net.Dial("tcp", portAddress); err == nil {
			conn.Close()
			logger.InfoWithPrefix(prefix, "Port %d is ready for connection", portNumber)
			return nil
		}

		// Log status update every 5 seconds
		if time.Since(lastLogTime) >= 5*time.Second {
			logRemainingTime(prefix, portNumber, endTime)
			lastLogTime = time.Now()
		}

		time.Sleep(time.Second)
	}

	return fmt.Errorf(
		"Timeout reached after %s seconds waiting for port %d to become ready for connection",
		utils.FormatDurationValue(timeoutSeconds),
		portNumber,
	)
}

// Helper function to log remaining time
func logRemainingTime(prefix string, portNumber int, endTime time.Time) {
	remainingSeconds := time.Until(endTime).Seconds()
	logger.InfoWithPrefix(
		prefix,
		"Checking if port %d is ready for connection... (timeout in %s seconds)",
		portNumber,
		utils.FormatDurationValue(remainingSeconds),
	)
}

// WaitForMultiplePorts waits for multiple ports to become available
func WaitForMultiplePorts(portNumbers []int, getContextPrefix func() string, timeoutSeconds float64) error {
	for _, portNumber := range portNumbers {
		if err := VerifyPortAvailability(portNumber, timeoutSeconds, getContextPrefix); err != nil {
			return err
		}
	}
	return nil
}

// ParsePortConfiguration converts various configuration formats into structured port data
// Returns: port numbers slice, timeout in seconds, and any error encountered
func ParsePortConfiguration(portConfig any) ([]int, float64, error) {
	const defaultTimeout = 30.0

	// Handle single port
	if port, ok := utils.ToInt(portConfig); ok {
		return []int{port}, defaultTimeout, nil
	}

	// Handle port list
	if portList, ok := portConfig.([]any); ok {
		return parsePortList(portList)
	}

	// Handle complex configuration
	if configMap, ok := portConfig.(map[string]any); ok {
		return parseConfigMap(configMap)
	}

	return nil, 0, fmt.Errorf("Parameter `awaits` must be a list of port numbers, or have the nested fields `ports` or `timeout`")
}

// Helper function to parse port list
func parsePortList(portList []any) ([]int, float64, error) {
	portNumbers := make([]int, len(portList))

	for i, val := range portList {
		if port, ok := utils.ToInt(val); ok {
			portNumbers[i] = port
		} else {
			return nil, 0, fmt.Errorf("Invalid port specification in `awaits`: %v", val)
		}
	}

	return portNumbers, 30, nil
}

// Helper function to parse configuration map
func parseConfigMap(configMap map[string]any) ([]int, float64, error) {
	var portNumbers []int
	timeoutSeconds := 30.0

	// Parse ports
	if portsConfig, exists := configMap["ports"]; exists {
		var err error
		portNumbers, err = extractPorts(portsConfig)
		if err != nil {
			return nil, 0, err
		}
	}

	// Parse timeout
	if timeoutConfig, exists := configMap["timeout"]; exists {
		if timeout, ok := utils.ToFloat64(timeoutConfig); ok {
			timeoutSeconds = timeout
		} else {
			return nil, 0, fmt.Errorf("Parameter `awaits.timeout` must be a number")
		}
	}

	return portNumbers, timeoutSeconds, nil
}

// Helper function to extract ports from configuration
func extractPorts(portsConfig any) ([]int, error) {
	// Single port
	if port, ok := utils.ToInt(portsConfig); ok {
		return []int{port}, nil
	}

	// Port list
	if portList, ok := portsConfig.([]any); ok {
		portNumbers := make([]int, len(portList))

		for i, val := range portList {
			if port, ok := utils.ToInt(val); ok {
				portNumbers[i] = port
			} else {
				return nil, fmt.Errorf("Invalid port specification in `awaits.ports`: %v", val)
			}
		}

		return portNumbers, nil
	}

	return nil, fmt.Errorf("Parameter `awaits.ports` must be a list of port numbers")
}
