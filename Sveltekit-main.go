package main
import (
    "encoding/json"
    "os/exec"
    "log"
    "net/http"
    "sync"
    "time"
)

type StreamManager struct {
    mu              sync.Mutex
    isActive        bool
    lastHeartbeat   time.Time
    timeoutDuration time.Duration
}

type Response struct {
    Status  string `json:"status"`
    Message string `json:"message"`
    URL     string `json:"url,omitempty"`
}

func NewStreamManager() *StreamManager {
    sm := &StreamManager{
        timeoutDuration: 5 * time.Minute,
    }

    // Start monitoring routine
    go sm.monitor()

    return sm
}

// Add CORS middleware
func enableCors(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Set CORS headers
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type")

        // Handle preflight
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }

        // Call the actual handler
        next(w, r)
    }
}

func (sm *StreamManager) stopService() error {
    log.Printf("Attempting to stop camera-stream.service...")
    // First try to stop service normally
    cmd := exec.Command("systemctl", "stop", "camera-stream.service")
    output, err := cmd.CombinedOutput()
    if err != nil {
        log.Printf("Error stopping service: %v, output: %s", err, string(output))
        return err
    }

    // Double check if ffmpeg is still running
    time.Sleep(2 * time.Second)  // Give it a moment
    checkCmd := exec.Command("pgrep", "ffmpeg")
    if output, _ := checkCmd.CombinedOutput(); len(output) > 0 {
        log.Printf("FFmpeg still running, forcing kill...")
        killCmd := exec.Command("pkill", "-9", "ffmpeg")
        if err := killCmd.Run(); err != nil {
            log.Printf("Error force killing ffmpeg: %v", err)
        }
    }

    log.Printf("Service stop command completed successfully")
    return nil
}

func (sm *StreamManager) isServiceRunning() (bool, error) {
    cmd := exec.Command("systemctl", "is-active", "camera-stream.service")
    output, err := cmd.CombinedOutput()
    if err != nil {
        return false, nil  // Service is not running
    }
    return string(output) == "active\n", nil
}

func (sm *StreamManager) InitiateStream(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        log.Printf("Method not allowed: %s", r.Method)
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    sm.mu.Lock()
    defer sm.mu.Unlock()

    // Check if already active
    if sm.isActive {
        log.Printf("Stream already active, returning existing URL")
        resp := Response{
            Status:  "success",
            Message: "Stream already active",
            URL:     "/playlist.m3u8",
        }
        sendJSONResponse(w, resp)
        return
    }

    log.Printf("Starting new stream...")
    // Start the service
    if err := sm.startService(); err != nil {
        log.Printf("Failed to start service: %v", err)
        http.Error(w, "Failed to start stream", http.StatusInternalServerError)
        return
    }

    sm.isActive = true
    sm.lastHeartbeat = time.Now()

    log.Printf("Stream started successfully")
    resp := Response{
        Status:  "success",
        Message: "Stream initiated",
        URL:     "/playlist.m3u8",
    }
    sendJSONResponse(w, resp)
}

func (sm *StreamManager) Heartbeat(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    sm.mu.Lock()
    defer sm.mu.Unlock()

    if !sm.isActive {
        http.Error(w, "No active stream", http.StatusNotFound)
        return
    }

    sm.lastHeartbeat = time.Now()

    resp := Response{
        Status:  "success",
        Message: "Heartbeat received",
    }
    sendJSONResponse(w, resp)
}

func (sm *StreamManager) monitor() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        sm.mu.Lock()
        if sm.isActive && time.Since(sm.lastHeartbeat) > sm.timeoutDuration {
            log.Println("Stream timed out, stopping service...")
            if err := sm.stopService(); err != nil {
                log.Printf("Error stopping service: %v", err)
            }
            sm.isActive = false
        }
        sm.mu.Unlock()
    }
}

func sendJSONResponse(w http.ResponseWriter, resp Response) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func main() {
    sm := NewStreamManager()

    http.HandleFunc("/initiate", enableCors(sm.InitiateStream))
    http.HandleFunc("/heartbeat", enableCors(sm.Heartbeat))

    log.Println("Server starting on :8080...")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
