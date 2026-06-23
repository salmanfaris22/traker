package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	TCP_PORT  = ":8080"
	HTTP_PORT = ":8090"
)

// -------- DATA STRUCTURE ----------
type Attendance struct {
	Data string `json:"data"`
	Time string `json:"time"`
}

var (
	mu       sync.Mutex
	logStore []Attendance
)

// -------- FILE LOG ----------
func logToFile(data string) {
	f, err := os.OpenFile("device_logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("File error:", err)
		return
	}
	defer f.Close()

	f.WriteString(time.Now().Format("2006-01-02 15:04:05") + " => " + data + "\n")
}

// -------- MEMORY STORE ----------
func saveLog(data string) {
	mu.Lock()
	defer mu.Unlock()

	logStore = append(logStore, Attendance{
		Data: data,
		Time: time.Now().Format("2006-01-02 15:04:05"),
	})
}

// -------- TCP HANDLER ----------
func handleConnection(conn net.Conn) {
	defer conn.Close()

	fmt.Println("🔌 Device connected:", conn.RemoteAddr())

	buffer := make([]byte, 4096)

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			fmt.Println("❌ Connection closed:", conn.RemoteAddr())
			return
		}

		if n > 0 {
			rawData := string(buffer[:n])

			fmt.Println("🔥 RAW DATA:")
			fmt.Println(rawData)

			saveLog(rawData)
			logToFile(rawData)
		}
	}
}

// -------- API: ATTENDANCE ----------
func getAttendance(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logStore)
}

// -------- API: HEALTH ----------
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","time":"%s"}`,
		time.Now().Format("2006-01-02 15:04:05"),
	)
}

// -------- MAIN ----------
func main() {

	// TCP SERVER
	go func() {
		listener, err := net.Listen("tcp", TCP_PORT)
		if err != nil {
			fmt.Println("TCP start error:", err)
			return
		}

		fmt.Println("🚀 TCP Server running on", TCP_PORT)

		for {
			conn, err := listener.Accept()
			if err != nil {
				fmt.Println("Accept error:", err)
				continue
			}

			go handleConnection(conn)
		}
	}()

	// HTTP SERVER
	go func() {
		http.HandleFunc("/attendance", getAttendance)
		http.HandleFunc("/health", healthCheck)

		fmt.Println("🌐 API Server running on", HTTP_PORT)

		err := http.ListenAndServe(HTTP_PORT, nil)
		if err != nil {
			fmt.Println("HTTP server error:", err)
		}
	}()

	// keep process alive
	select {}
}
