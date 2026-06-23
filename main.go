package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

const PORT = ":8080"

// -------- DATA STORE ----------
type Attendance struct {
	Data string
	Time string
}

var (
	mu       sync.Mutex
	logStore []Attendance
)

// -------- LOG TO FILE ----------
func logToFile(data string) {
	f, err := os.OpenFile("device_logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("File error:", err)
		return
	}
	defer f.Close()

	f.WriteString(time.Now().Format("2006-01-02 15:04:05") + " => " + data + "\n")
}

// -------- STORE IN MEMORY ----------
func saveLog(data string) {
	mu.Lock()
	defer mu.Unlock()

	log := Attendance{
		Data: data,
		Time: time.Now().Format("2006-01-02 15:04:05"),
	}

	logStore = append(logStore, log)
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

			fmt.Println("🔥 RAW PACKET RECEIVED:")
			fmt.Println(rawData)

			// store logs
			saveLog(rawData)
			logToFile(rawData)

			// TODO: parse eSSL format here
		}
	}
}

// -------- API HANDLER ----------
func getAttendance(w http.ResponseWriter, r *http.Request) {

	mu.Lock()
	defer mu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	fmt.Fprintf(w, "[")

	for i, log := range logStore {
		fmt.Fprintf(w,
			`{"time":"%s","data":"%s"}`,
			log.Time, log.Data,
		)

		if i < len(logStore)-1 {
			fmt.Fprintf(w, ",")
		}
	}

	fmt.Fprintf(w, "]")
}

// -------- MAIN ----------
func main() {

	// TCP SERVER
	go func() {
		listener, err := net.Listen("tcp", PORT)
		if err != nil {
			fmt.Println("TCP start error:", err)
			return
		}

		fmt.Println("🚀 TCP Server running on port", PORT)

		for {
			conn, err := listener.Accept()
			if err != nil {
				continue
			}

			go handleConnection(conn)
		}
	}()

	// HTTP API SERVER (same port 8080 NOT possible → use 8090 API OR separate handler)
	go func() {
		http.HandleFunc("/attendance", getAttendance)

		fmt.Println("🌐 API Server running on :8090")
		http.ListenAndServe(":8090", nil)
	}()

	// keep main alive
	select {}
}
