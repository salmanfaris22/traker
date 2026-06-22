// facetracker is a tiny standalone listener for ZKTeco face-attendance
// devices (the "iclock" push protocol). It accepts the device's attendance
// uploads, keeps the parsed punches IN MEMORY (no database), and exposes a
// single read-only endpoint:
//
//	GET /api/logs   ->  JSON list of every punch received since startup
//
//	go run .        # listens on :8080
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Punch is one attendance record parsed from an ATTLOG upload.
type Punch struct {
	DeviceSN string `json:"device_sn"`
	UserID   string `json:"user_id"`
	Time     string `json:"time"` // RFC3339 UTC, e.g. "2026-06-19T12:09:01Z" ("" if unparsable)
	Raw      string `json:"raw"`  // original tab-delimited line from the device
}

// store is an in-memory, concurrency-safe buffer of punches (no DB).
type store struct {
	mu      sync.RWMutex
	punches []Punch
}

func (s *store) add(p Punch) {
	s.mu.Lock()
	s.punches = append(s.punches, p)
	s.mu.Unlock()
}

// list returns a copy in the order punches were received.
func (s *store) list() []Punch {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Punch, len(s.punches))
	copy(out, s.punches)
	return out
}

var punches = &store{}

func main() {
	mux := http.NewServeMux()

	// Device side (ZKTeco / iclock push protocol).
	mux.HandleFunc("/iclock/cdata.aspx", handleCdata)
	mux.HandleFunc("/iclock/getrequest.aspx", handleGetRequest)
	mux.HandleFunc("/iclock/facephoto.aspx", handleFacePhoto)
	mux.HandleFunc("/iclock/snapshot.aspx", handleFacePhoto)
	mux.HandleFunc("/iclock/verifyphoto.aspx", handleFacePhoto)

	// Read-only API: the punches collected so far.
	mux.HandleFunc("/api/logs", handleLogs)

	addr := ":8080"
	fmt.Println("Listening on", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Println("server error:", err)
	}
}

// ========================= GET API =========================

// handleLogs returns every punch received so far as a JSON array.
func handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	list := punches.list()
	if list == nil {
		list = []Punch{} // emit [] rather than null when empty
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(list); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ========================= ATTENDANCE UPLOAD =========================

// handleCdata receives an ATTLOG upload, parses each line into a Punch and
// buffers it. The device serial arrives as the ?SN= query param.
func handleCdata(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	sn := r.URL.Query().Get("SN")

	for _, line := range strings.Split(string(body), "\n") {
		p, ok := parsePunch(line, sn)
		if !ok {
			continue
		}
		punches.add(p)
		fmt.Printf("🧾 Punch -> SN:%s User:%s Time:%s\n", p.DeviceSN, p.UserID, p.Time)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// parsePunch turns one ATTLOG line into a Punch.
//
// ZKTeco ATTLOG is tab-delimited:  PIN \t "YYYY-MM-DD HH:MM:SS" \t status \t verify \t ...
// Some firmware separates everything with spaces, so we fall back to that:
// PIN  YYYY-MM-DD  HH:MM:SS  ...
func parsePunch(line, sn string) (Punch, bool) {
	raw := strings.Trim(line, "\r\n ")
	if raw == "" {
		return Punch{}, false
	}

	var userID, dt string
	if cols := strings.Split(raw, "\t"); len(cols) >= 2 && strings.Contains(cols[1], " ") {
		userID = strings.TrimSpace(cols[0])
		dt = strings.TrimSpace(cols[1])
	} else {
		f := strings.Fields(raw)
		if len(f) < 3 {
			return Punch{}, false
		}
		userID = f[0]
		dt = f[1] + " " + f[2]
	}

	iso := ""
	if t, err := time.Parse("2006-01-02 15:04:05", dt); err == nil {
		iso = t.UTC().Format(time.RFC3339)
	}

	return Punch{DeviceSN: sn, UserID: userID, Time: iso, Raw: raw}, true
}

// ========================= HEARTBEAT =========================

// handleGetRequest answers the device's command poll. We have no commands to
// push, so a bare OK keeps it happy.
func handleGetRequest(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// ========================= FACE PHOTO =========================

// handleFacePhoto accepts (and discards) face snapshots so the device does not
// retry. Photos are not stored — this service only serves the punch log list.
func handleFacePhoto(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(20 << 20); err == nil {
			if file, header, err := r.FormFile("image"); err == nil {
				defer file.Close()
				data, _ := io.ReadAll(file)
				fmt.Printf("📸 Face photo %s (%d bytes) from SN:%s\n", header.Filename, len(data), r.URL.Query().Get("SN"))
			}
		}
	} else if body, _ := io.ReadAll(r.Body); len(body) > 0 {
		fmt.Printf("📸 Face photo (raw, %d bytes) from SN:%s\n", len(body), r.URL.Query().Get("SN"))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
