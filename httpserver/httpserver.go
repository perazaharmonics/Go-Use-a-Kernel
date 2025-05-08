/*
==============================================================================
* Filename: httpserver.go
* Description: An implmenetation of a simple HTTP server that listens for
* incoming HTTP request and serves them with a simple response. These responses
* are in regard to the state of the Proxy Server pod cached in the system. It
* contains handlers for various endpoints, such as:
* metrics, health, status, liveness, readiness, ping, and version.
*
* Author:
*  J.EP J. Enrique Peraza
==============================================================================
*/
package httpserver

import (
	logger "github.com/perazaharmonics/project_name/internal/logger"              // Our custom log package.
	"github.com/perazaharmonics/project_name/config"         // Our configuration file
	"github.com/perazaharmonics/project_name/internal/utils" // Our Handlers and Callbacks functions
	"bytes"                            // For byte buffer operations.
	"bufio"                            // For buffered I/O
	"strings"                          // For string manipulation
	"context"                          // For context management
	"encoding/json"                    // For JSON encoding and decoding
	"fmt"                              // For formatted I/O
	"net/http"                         // For HTTP server and client
	"os"                               // For file operations, I/O, system calls
	"os/exec"                          // For executing external commands
	"strconv"                          // For string conversion
	"sync"                             // For mutexes and locks
	"sync/atomic"                      // For atomic operations
	"time"                             // For time and duration
)

var log = logger.NewLogger()            // Logger instance for logging
//const debug = true                    // Enables debug logging.
var execCommandContext = exec.CommandContext // For executing external commands with context.
// ------------------------------------ //
// Helper function to write into a byte buffer.
// it takes a string as an argument and concatenates it into the buffer passed
// to the function. It returns an error if any error occurs while writing to the
// ------------------------------------ //
func PackBuffer(msg string, buf *bytes.Buffer) (len int, err error) {
	if _, err := buf.WriteString(msg); err != nil { // Error writing to buffer?
		return 0, err                       // Yes, return 0 and the error.
	}                                     // Done writing to the buffer.
	len = buf.Len()                       // Get the length of the buffer.
	return len, nil                       // Return buf length and nil error.
}                                       // ---------- PackBuffer ----------- //
type HttpServer struct {                // HTTP server object.
	port    int                           // The port to make requests from.
	now     time.Time                     // This instant.
	lpwt    time.Duration                 // Liveness Probe wait time.
	rpwt    time.Duration                 // Readiness probe wait time.
	isready bool                          // Flag to signal server readiness.
	mapmtx  sync.RWMutex                  // Mtx to protect concurrent reloads  of mappings.
	maps    []config.Mapping              // The mappings from the config file.
	cfgp    string                        // The path to the config file.
	vrs     string                        // The version set at build time.
	rscript string                        // The path to the reset script.
	ncnx    uint64                        // The number of connection to the server.
	chit    uint64                        // The number of cache hits.
	cmiss   uint64                        // The number of chache misses.
	nload   uint64                        // The number of reloads.
} // The HTTP server object.
// ------------------------------------ //
// NewHttpServer creates a new HTTP server object.
// It takes the port number, the path to the config file, and the version
// as arguments. It returns a pointer to the HTTP server object.
// ------------------------------------ //
func NewHttpServer(p int, c, v string) *HttpServer {
	if v == "" {	                        // Was the 'v' field empty?
		vdd := "/home/ljt/Projects/NetGo/ProxyServer/Proxyd.vdd"
	  f, err := os.Open(vdd)                // Open the version file.
		if err != nil {                     // Could we open the version file?
			log.Err("Could not open version file %s: %v", vdd, err)
		} else {                            // Else we could open the vdd.
			defer f.Close()                   // Close the file when done.
			scanner:=bufio.NewScanner(f)      // Create a new scanner to read file.
			foundsection:=false               // Flag to signal if we found section
			for scanner.Scan() {              // Read the file line by line.
				line:=strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line,"[Version]") {
					foundsection=true             // We found the target section.
					continue                      // So continue to the next line.
				}                               // Done with if we found section.
				if foundsection && strings.HasPrefix(line,"version=") {
					v=strings.TrimSpace(strings.TrimPrefix(line,"version="))
					break                         // Get the value of the version key.
				}                               // Done with if we found the version key.
			}                                 // Done reading the file line by line.
			if err:=scanner.Err();err!=nil{   // Error reading the file?
				log.Err("Error reading version file %s: %v", vdd, err)
			}                                 // Yes, log the error.
			if v!="" {                        // Did we populate the version?
				log.Inf("Version from file %s: %s", vdd, v) // Yes, log the version.
			} else {                          // No, we could not find the version.
				log.War("Could not find version in file %s", vdd) // No, log the warning.
				log.Inf("The version will be set by the Makefile.")
			}                                 // Done with if we could populate the version.                           
		}                                   // Done with the version file.                                
	}                                                       
	return &HttpServer{                   // Return a new HTTP server object.
		port:    p,                         // The port to make requests from.
		now:     time.Now(),                // This instant.
		lpwt:    5 * time.Second,           // Liveness probe wait time.
		rpwt:    15 * time.Second,          // Readiness probe wait time.
		isready: false,                     // Flag to signal server readiness.
		mapmtx:  sync.RWMutex{},            // Mtx to protect concurrent reloads of mappings.
		maps:    nil,                       // The mappings from the config file.
		cfgp:    c,                         // The path to the config file.
		vrs:     v,                         // The version set at build time.
		rscript: "/home/ljt/Projects/NetGo/logs/reset.sh",
		ncnx:    0,                         // The number of connections to the server.
		chit:    0,                         // The number of cache hits.
		cmiss:   0,                         // The number of cache misses.
		nload:   0,                         // The number of reloads.
	}                                     // Done initializing the HTTP server object.
}                                       // ------------ NewHttpServer ------- //
// ------------------------------------ //
// Functions to set/get the server port.
// It takes the port number as an argument or returns the port number.
// ------------------------------------ //
func (s *HttpServer) SetPort(p int) {
	s.port = p                            // Set the port number/
}                                       // ---------- SetPort --------------- //
func (s *HttpServer) GetPort() int {
	return s.port                         // Return the port number.
}                                       // --------- GetPort ---------------- //
// ------------------------------------ //
// Functions to set/get the time the server started
// It takes the time as an argument or returns the time.
// ------------------------------------ //
func (s *HttpServer) SetTime(t time.Time) {
	s.now = t                             // Set the time the server started.
}                                       // --------- SetTime ---------------- //
func (s *HttpServer) GetTime() time.Time {
	return s.now                          // Return the time the server started.
}                                       // -------- GetTime ----------------- //
// ------------------------------------ //
// Functions to set/get programmatically the Liveness and Readiness probe wait times.
// It takes the wait time as an argument or returns the wait time.
// ------------------------------------ //
func (s *HttpServer) SetLivenessWaitTime(t time.Duration) {
	s.lpwt = t                            // Set the liveness probe wait time.
}                                       // ---- SetLivenessWaitTime -------- //
func (s *HttpServer) GetLivenessWaitTime() time.Duration {
	return s.lpwt                         // Return the liveness probe wait time.
}                                       // -- GetLivenessWaitTime --------- //
func (s *HttpServer) SetReadinessWaitTime(t time.Duration) {
	s.rpwt = t                            // Set the readiness probe wait time.
}                                       // -- SetReadinessWaitTime --------- //
func (s *HttpServer) GetReadinessWaitTime() time.Duration {
	return s.rpwt                         // Return the readiness probe wait time.
}                                       // -- GetReadinessWaitTime --------- //
// ------------------------------------ //
// Functions to set/get the server readiness flag.
// It takes the readiness flag as an argument or returns the readiness flag.
// ------------------------------------ //
func (s *HttpServer) SetReady(r bool) {
	s.isready = r                         // Set the readiness flag.
}                                       // --------- SetReady --------------- //
func (s *HttpServer) GetReady() bool {
	return s.isready                      // Return the readiness flag.
}                                       // -------- GetReady ---------------- //
// ------------------------------------ //
// Get/Set the version number of the server.
// ------------------------------------ //
func (s *HttpServer) SetVersion(v string) { // Set the version of the server.
	s.vrs = v                             // Set the version of the server.
}                                       // --------- SetVersion ------------- //
func (s *HttpServer) GetVersion() string {
	return s.vrs                          
}                                       // -------- GetVersion -------------- //
// ------------------------------------ //
// Set the configuration file path.
// This is the path to the YAML file that contains the mappings.
// ------------------------------------ //
func (s *HttpServer) SetConfigPath(c string) { // Set the config file path.
	s.cfgp = c                            // Set the config file path.
}                                       // --------- SetConfigPath ---------- //
// ------------------------------------ //
// Get the configuration file path.
// This is the path to the YAML file that contains the mappings.
// ------------------------------------ //
func (s *HttpServer) GetConfigPath() string { // Get the config file path.
	return s.cfgp                         // Return the config file path.
}                                       // --------- GetConfigPath ---------- //
// ------------------------------------ //
// Get the mappings from the configuration file.
// This is the path to the YAML file that contains the mappings.
// ------------------------------------ //
func (s *HttpServer) GetMappings() []config.Mapping { // Get the mappings.
	return s.maps                         // Return the mappings.
}                                       // --------- GetMappings ------------ //
// ------------------------------------ //
// Function to get the number of connections to the server.
// ------------------------------------ //
func (s *HttpServer) GetConnections() uint64 { // Get the number of connections.
	return atomic.LoadUint64(&s.ncnx)     // Return the number of connections.
}                                       // --------- GetConnections --------- //
// ------------------------------------ //
// Functions to get the number of cache hits/misses
// ------------------------------------ //
func (s *HttpServer) GetCacheHits() uint64 {
	return atomic.LoadUint64(&s.chit)     // Return the number of cache hits.
}                                       // --------- GetCacheHits ----------- //

func (s *HttpServer) GetCacheMisses() uint64 {
	return atomic.LoadUint64(&s.cmiss)    // Return the number of cache misses.
}                                       // --------- GetCacheMisses ---------- //

// ------------------------------------ //
// Increment the number of connections to the server.
// This is done atomically to avoid contention between threads.
// ------------------------------------ //
func (s *HttpServer) IncrementConnections() {
	atomic.AddUint64(&s.ncnx, 1)          // Increment the number of connections.
}                                       // --------- IncrementConnections ---- //
// ------------------------------------ //
// Decrement the number of connections to the server.
// This is done atomically to avoid contention between threads.
// The decrement is done by adding 2^64 - 1 to the current value.
// This is a trick to avoid overflow when decrementing.
// The result is equivalent to n - 1, where n is the current value.
// ------------------------------------ //
func (s *HttpServer) DecrementConnections() {
	// Equivalent to n + (2^64 - 1) = (n -1)*(mod 2 ^64)= n - 1
	// This is a trick to avoid overflow when decrementing.
	atomic.AddUint64(&s.ncnx, ^uint64(0))
}                                       // ------- DecrementConnections ---- //
// ------------------------------------ //
// Increment the number of cache hits.
// This is done atomically to avoid contention between threads.
// ------------------------------------ //
func (s *HttpServer) IncrementCacheHits() {
	atomic.AddUint64(&s.chit, 1)         // Increment the number of cache hits.
}                                      // -------- IncrementCacheHits ------- //
// ----------------------------------- //
// Increment the number of cache misses.
// This is done atomically to avoid contention between threads.
// ------------------------------------ //
func (s *HttpServer) IncrementCacheMisses() {
	atomic.AddUint64(&s.cmiss, 1)         // Increment the number of cache misses.
}                                       // ------- IncrementCacheMisses ----- //
// ----------------------------------- //
// Increment the number of mapping reloads.
// This is done atomically to avoid contention between threads.
// ------------------------------------ //
func (s *HttpServer) IncrementReloads() {
	atomic.AddUint64(&s.nload, 1)         // Increment the number of reloads.
}                                       // ------- IncrementReloads --------- //
// ------------------------------------ //
// Get the environment variable value from the proxyd deployment.
// ------------------------------------ //
func (s *HttpServer) getEnvTimes(t string) (d time.Duration, err error) {
	var ev int                            // The env variable for durations.
	ev, err = strconv.Atoi(os.Getenv(t))  // Convert the env var to integer.
	d = time.Duration(ev) * time.Second   // Convert that timespan to seconds.
	return d, err                         // Return value and error code.
}                                       // ---------- getEnvTimes ----------- //
// ------------------------------------ //
// Set the environment variable times for liveness and readiness probes.
// ------------------------------------ //
func (s *HttpServer) SetEnvTimes() (err error) {
	s.lpwt, err = s.getEnvTimes("WAIT_LIVENESS_TIME")
	if err != nil {                       // Defined the liveness probe time?
		log.War("Environment variable \"WAIT_LIVENESS_TIME\" not set. %v", err)
		log.Inf("Setting \"WAIT_LIVENESS_TIME\" to default of 5s.")
		s.lpwt = 5 * time.Second            // No, set to default at 5s.
	}                                     // Done with liveness probe time not defined.
	s.rpwt, err = s.getEnvTimes("WAIT_READINESS_TIME")
	if err != nil {                       // Defined the readiness probe time?
		log.War("Environment variable \"WAIT_READINESS_TIME\" not set. %v", err)
		log.Inf("Setting \"WAIT_READINESS_TIME\" to default of 15s.")
		s.rpwt = 15 * time.Second           // No, set to default at 15 seconds.
	}                                     // Done with readiness probe time not defined.
	return err                            // Return the error code.
}                                       // --------- setEnvTimes ------------ //

// Allow for the tests to be overriden with a dummy stub script.
func (s *HttpServer) SetRotateScript(path string) {
  s.rscript=path
}																			 // --------- SetRotateScript -------- //
func (s *HttpServer) GetRotateScript() string {
	return s.rscript
}

// ----------------------------------- //
// Liveness probe handler verifies if the server is alive.
// It checks if the server has been running for longer than the
// liveness probe wait time (lpwt). If it has, it returns an
// HTTP OK status. If not, it returns an HTTP Service Unavailable status.
// ------------------------------------ //
func (s *HttpServer) LivenessProbe(w http.ResponseWriter, r *http.Request) {
	log.Inf("Received \"healthz\" request from %s", r.RemoteAddr)
	if time.Since(s.now) > s.lpwt {       // Have we waited longer than lpwt?
		w.WriteHeader(http.StatusOK)        // Yes, so write HTTP OK header.
		w.Write([]byte("OK"))               // And send a OK msg to K8.
		log.Inf("Sent \"OK\" status message to HTTP server.")
	} else {                              // Else we have not waited that long.
		w.WriteHeader(http.StatusServiceUnavailable) // Say we are unavailable.
		w.Write([]byte("Not OK"))           // Send a Not OK msg to K8s API.
		log.Inf("Sent \"Not OK\" status message to HTTP server.")
	}                                     // Done with not waiting enough.
}                                       // -------- livenessProbe ----------- //
// ------------------------------------ //
// Readiness probe handler verifies if the server is ready.
// It checks if the server has been running for longer than the
// readiness probe wait time (rpwt). If it has, it returns an
// HTTP OK status. If not, it returns an HTTP Service Unavailable status.
// ------------------------------------ //
func (s *HttpServer) ReadinessProbe(w http.ResponseWriter, r *http.Request) {
	log.Inf("Received \"readyz\" request from %s", r.RemoteAddr)
	// Did we manually mark the server as ready?
	if s.isready {
	  w.WriteHeader(http.StatusOK)        // Yes, so write HTTP OK header.
		w.Write([]byte("Ready"))            // and send a ready msg to K8s API.
		log.Inf("Sent \"Ready\" status message to HTTP server (manual readiness).")
		return
	}
	// Have we waited longer than rpwt, and has the previous job finished?
	if time.Since(s.now) > s.rpwt {
		w.WriteHeader(http.StatusOK)        // Yes, so write HTTP OK header.
		w.Write([]byte("Ready"))            // and send a ready msg to K8s API.
		log.Inf("Sent \"Ready\" status message to HTTP server.")
	} else {                              // Else we have not waited that long.
		w.WriteHeader(http.StatusServiceUnavailable) // Say we are unavailable.
		w.Write([]byte("Not Ready"))        // Send a Not Ready msg to K8s API.
		log.Inf("Sent \"Not Ready\" status message to HTTP server")
	}                                     // Done with not waiting enough.
}                                       // -------- readinessProbe ---------- //
// ------------------------------------ //
// Ping probe handler verifies if the server is alive.
// It checks if the server is alive and returns an HTTP OK status.
// ------------------------------------ //
func (s *HttpServer) PingProbe(w http.ResponseWriter, r *http.Request) {
	log.Inf("Received \"pingz\" request from %s", r.RemoteAddr)
	w.WriteHeader(http.StatusOK) // Write HTTP OK header.
	w.Write([]byte("pong"))      // Notify Kubernetes API we are listening.
	log.Inf("Sent \"pong\" as a response to ping request.")
} // ------------ ping ---------------- //
// ------------------------------------ //
// Version probe handler verifies the version of the Proxy Server application
// that is currently running. It returns the version of the application
// as a response to the Kubernetes API.
// ------------------------------------ //
func (s *HttpServer) VersionProbe(w http.ResponseWriter, r *http.Request) {
	log.Inf("Received \"versionz\" request from %s", r.RemoteAddr)
	w.WriteHeader(http.StatusOK)          // Write HTTP OK header.
	fmt.Fprintf(w, "Proxy Version: %s\n", s.vrs) // Send the version to K8s API.
	log.Inf("Sent \"version\" as a response to version request.")
}                                       // ----------- versionHandler ------- //
// ------------------------------------ //
// Status probe handler verifies the status of the server.
// It checks if the server is ready and returns an HTTP OK status.
// It also returns the liveness and readiness probe wait times.
// ------------------------------------ //
func (s *HttpServer) StatusProbe(w http.ResponseWriter, r *http.Request) {
	log.Inf("Received \"statusz\" request from %s", r.RemoteAddr)
	if s.isready {                        // Is the server ready?
		w.WriteHeader(http.StatusOK)        // Yes, write OK hdr and provide status.
		fmt.Fprintf(w, "Proxyd up since: %s\nLiveness delay: %s\nReadiness delay: %s\n",
			s.now.Format(time.RFC3339Nano), s.lpwt, s.rpwt)
		log.Inf("Sent status message to HTTP server.")
	} else {                              // Else the server is not ready.
		w.WriteHeader(http.StatusServiceUnavailable) // Say we are unavailable.
		w.Write([]byte("Server status unavailable."))
	}                                     // Done with server not ready .
}                                       // ---------- statusHandler --------- //
// ------------------------------------ //
// Map probe handler verifies the mappings that the Proxy Server is currently
// using. It returns the mappings as a JSON response to the Kubernetes API.
// ------------------------------------ //
func (s *HttpServer) MapProbe(w http.ResponseWriter, r *http.Request) {
	log.Inf("Received a \"mapz\" request from %s", r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json") // Set the content type to JSON
	s.mapmtx.RLock()                      // Lock mutex for exclusive read access.
	defer s.mapmtx.RUnlock()              // Unlock the mutex when done.
	json.NewEncoder(w).Encode(s.maps)     // Encode the mappings to JSON and send to K8s API.                                     // Done printing the mappings.
	log.Inf("Sent pod mapping as a response to map request.")
}                                       // ----------- mapHandler ------------ //
// ------------------------------------ //
// Reload probe handler verifies the mappings that the Proxy Server is currently
// using. It reloads the mappings from the configuration file and returns
// the mappings as a JSON response to the Kubernetes API.
// It spawns a goroutine to reload the mappings concurrently.
// ------------------------------------ //
func (s *HttpServer) ReloadProbe(w http.ResponseWriter, r *http.Request) {
	log.Inf("Received a \"reloadz\" request from %s", r.RemoteAddr)
	// ---------------------------------- //
	// Spawn a goroutine to reload the config file concurrently.
	// This is done to avoid blocking the main thread.
	// ---------------------------------- //
	go func() {                           // Start a new goroutine to reload cfg file.
		if err:=s.LoadMappings();err!=nil { // Could we reload the config file?
		  log.Err("Could not reload config file %s: %v", s.cfgp, err)
			http.Error(w, "Could not reload config file.", http.StatusInternalServerError)
			return                            // No, return.
		}                                   // Done checking for error loading cfg.
		atomic.AddUint64(&s.nload, 1)       // We have reloaded this many times.
	}()                                   // Done with goroutine to reload cfg file.
	w.WriteHeader(http.StatusAccepted)    // Write HTTP Accepted header.
	log.Inf("Reload initiated for config file %s", s.cfgp)
	w.Write([]byte("Reload initiated"))   // Send a reload init msg to K8s API.
}                                       // ---------- reloadHandler --------- //
// ------------------------------------ //
//  1. We execute reset.sh which runs CheckLogFile.go which sends a SIGHUP
//     signal to the programs which have the log file open.
//  2. The SIGHUP signal tells the program to close the log file.
//  3. The log file is opened again by using ExitLog() in the SignalHandler.
//  4. We notify that log rotation was successful by sending a message to
//
// the K8s API.
// ------------------------------------ //
func (s *HttpServer) RotateLogs(w http.ResponseWriter, r *http.Request) {
	log.Inf("Received log rotation request from %s", r.RemoteAddr)
	// ---------------------------------- //
	// Add a context with a timeout in case the script hangs.
	// It should not hang, but just in case.
	// ---------------------------------- //
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()                        // Cancel context when done.
	script := s.GetRotateScript()				 // Get the script to run.
	log.Inf("Executing log rotation script %s", script)
	cmd := execCommandContext(ctx, script)// Create a command with this ctx to run the script.
	if err := cmd.Run(); err != nil {     // Run cmd; handle error.
		http.Error(w, "Log rotation failed.", http.StatusInternalServerError)
		log.Err("Log rotation failed: %v", err)
		return                              // Yes, return. Error running script.
	}                                     // Done running script or handling err.
	if ctx.Err() == context.DeadlineExceeded { // Did the script timeout?
		http.Error(w, "Log rotation timed out.", http.StatusInternalServerError)
		log.Err("Log rotation script timed out.")
		return                              // Yes, return. Timed out resetting logs.
	}                                     // Done checking for timeout.
	w.WriteHeader(http.StatusAccepted)    // Write HTTP Accepted header.
	fmt.Fprintln(w, "Log rotation successful")// Send success message to K8s API.
	log.Inf("Log rotation completed successfully.")
}                                       // ---------- rotateLogs ------------ //
// ------------------------------------ //
// Metric probe handler verifies the metrics that the Proxy Server is currently
// using. It returns the metrics as a text/plain Prometheus format response to the K8s API.
// It returns the uptime, number of connections, cache hits, cache misses,
// and number of reloads as a response to the Kubernetes API monitoring
// the performance of the Proxy Server.
// ------------------------------------ //
func (s *HttpServer) MetricProbe(w http.ResponseWriter, r *http.Request) {
	log.Inf("Received \"metricz\" request from %s", r.RemoteAddr)
	now := time.Now()                     // Get the current time.
	uptime := now.Sub(s.now)              // The server's uptime.
	// ---------------------------------- //
	// Preallocate buffer size with enough space for the response message.
	// ---------------------------------- //
	buf := bytes.NewBuffer(make([]byte, 0, 1024)) // Preallocate buffer.
	// ---------------------------------- //
	// Concatenate the uptime metric to K8s API.
	// ---------------------------------- //
	msg := "# HELP proxyd_uptime_seconds Seconds since proxyd started\n"
	msg += "# TYPE proxyd_uptime_seconds counter\n"
	msg += fmt.Sprintf("proxyd_uptime_seconds %f\n", uptime.Seconds())
	log.Inf("Sent \"uptime\" as a response to metric request.")
	// ---------------------------------- //
	// Concatenate the number of TCP connections to K8s API.
	// ---------------------------------- //
	msg += "# HELP proxyd_connections_total Total number of TCP connections\n"
	msg += "# TYPE proxyd_connections_total counter\n"
	msg += fmt.Sprintf("proxyd_connections_total %d\n", atomic.LoadUint64(&s.ncnx))
	log.Inf("Sent \"connections\" as a response to metric request.")
	// ---------------------------------- //
	// Send the number of proxyd cache hits to K8s API.
	// ---------------------------------- //
	msg += "# HELP proxyd_cache_hits Cache hits when resolving pod IP addresses\n"
	msg += "#TYPE proxyd_cache_hits counter\n"
	msg += fmt.Sprintf("proxyd_cache_hits %d\n", atomic.LoadUint64(&s.chit))
	log.Inf("Sent \"cache hits\" as a response to metric request.")
	// ---------------------------------- //
	// Send the number of proxyd cache misses to K8s API.
	// ---------------------------------- //
	msg += "#HELP proxyd_cache_misses Cache misses when resolving pod IP addresses\n"
	msg += "#TYPE proxyd_cache_misses counter\n"
	msg += fmt.Sprintf("proxyd_cache_misses %d\n", atomic.LoadUint64(&s.cmiss))
	log.Inf("Sent \"cache misses\" as a response to metric request.")
	// ---------------------------------- //
	// Send the number of proxyd reloads to K8s API.
	// ---------------------------------- //
	msg += "#HELP proxyd_reloads Total number of config reloads\n"
	msg += "#TYPE proxyd_reloads counter\n"
	msg += fmt.Sprintf("proxyd_reloads %d\n", atomic.LoadUint64(&s.nload))
	log.Inf("Sent \"reloads\" as a response to metric request.")
	// ---------------------------------- //
	// Pack the buffer with the metrics response message.
	// ---------------------------------- //
	length, err := PackBuffer(msg, buf)   //
	if err != nil {                       // Error packing the buffer?
		log.Err("Failed to pack buffer for metrics response: %v", err)
		http.Error(w, "Failed to pack buffer for metrics response.", http.StatusInternalServerError)
		return                              // Yes, return we can't send metrics.
	}                                     // Done packing buf and checking error.
	// ---------------------------------- //
	// Set the Correct prometheus conent type header.
	// ---------------------------------- //
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", length))
	if _, err := w.Write(buf.Bytes()); err != nil {
		log.Err("Failed to write metrics response: %v", err)
		http.Error(w, "Failed to write metrics response.", http.StatusInternalServerError)
		return                              // Yes, return we can't send metrics.
	}                                     // Done writing the metrics response.
	log.Inf("Sent metrics response to K8s API.")
	log.Inf("Metrics response: %s", buf.String()) // Print the metrics response.
}                                       // ----------- MetricHandler -------- //
// ------------------------------------ //
// Function to launch the HTTP server. It takes as an argument the
// a multiplexer to handle the HTTP requests. It returns an error if any error
// occurs while starting the server othwerwise it returns nil.
// ------------------------------------ //
func (s *HttpServer) Start(mux *http.ServeMux) error {
	s.SetEnvTimes()                       // Set the environment variable times.
	addr := fmt.Sprintf(":%d", s.port)    // The address to listen on.
	srv := &http.Server{                  // The native GO HTTP server object.
		Addr:              addr,            // The address to listen on.
		Handler:           mux,             // The request hanlder is a multiplexer.
		MaxHeaderBytes:    1 << 20,         // 1 MiB max header size (experimental).
		ReadHeaderTimeout: 15 * time.Second,// Read header timeout.
		WriteTimeout:      20 * time.Second,// Write timeout.
		ReadTimeout:       20 * time.Second,// Read timeout.
	}                                     // Done with the HTTP server object.
	// ---------------------------------- //
	// Register the graceful shutdown callback function.
	// to clean up the server when it is stopped and handle shutdown gracefully.
	// ---------------------------------- //
	utils.RegisterShutdownCB(func() {     // Our exit handler.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()                      // Cancel the context when done.
		log.Inf("Shutting down HTTP server on port :%d", s.port)
		if err := srv.Shutdown(ctx); err != nil { // Error shutting down server?
			log.Err("Error shutting down HTTP server: %v", err) // Yes, log it.
		} else {                            // Else we are shutting down.
			log.Inf("HTTP server on port %d shut down successfully", s.port)
		}                                   // Done with shutting down the server.
	})                                    // Done registering exit handler.
	log.Inf("Starting HTTP server on port %d", s.port)
	return srv.ListenAndServe()           // Start the HTTP server.
}                                       // ------------ Start --------------- //
// ------------------------------------ //
// Function to read configuration and load mappings into the object.
// ------------------------------------ //
func (s *HttpServer) LoadMappings() error {
  conf,err:=config.ReadConfig(s.cfgp)   // Read the config file.
	if err!=nil {										      // Could we read the config file?
		log.Err("Could not read config file %s: %v", s.cfgp, err)
		return err                          // No, return the error.
	}                                     // Done with can't read config file.
	s.mapmtx.Lock()                       // Lock the mutex for writing.
	s.maps = conf.Mappings                // Load the mappings into the object.
	s.mapmtx.Unlock()     	              // Unlock the mutex.
	log.Inf("Loaded %d mappings from config file %s", len(s.maps), s.cfgp)
	return nil                            // Return nil, we are done.                          
}                                       // ----------- LoadMappings --------- //
// ------------------------------------ //
// Function to print the value of the fields in the http server object.
// It is used for debugging purposes.
// ------------------------------------ //
func (s *HttpServer) PrintObject() {
	log.Inf("HTTP Server object:")        // Print the HTTP server object.
	log.Inf("Port: %d", s.port)           // Print the port number.
	log.Inf("Time: %s", s.now.Format(time.RFC3339Nano)) // Print the time.
	log.Inf("Liveness wait time: %s", s.lpwt) // Print the liveness wait time.
	log.Inf("Readiness wait time: %s", s.rpwt) // Print the readiness wait time.
	log.Inf("Is ready: %t", s.isready)    // Print the readiness flag.
	log.Inf("Mappings: %v", s.maps)       // Print the mappings.
	log.Inf("Config path: %s", s.cfgp)    // Print the config path.
	log.Inf("Version: %s", s.vrs)         // Print the version.
	log.Inf("Number of connections: %d", s.ncnx) // Print the number of connections.
	log.Inf("Number of cache hits: %d",s.chit) // Print the number of cache hits.
	log.Inf("Number of cache misses: %d",s.cmiss) // Print the number of cache misses.
	log.Inf("Number of reloads: %d",s.nload) // Print the number of reloads.
	log.Inf("Mappings:")                  // Print the mappings.
	for _,m:=range s.maps{                // For the number of key/values in mapping.
	  if m.Alias==""{                     // Is the IP Address alias empty?
		  log.Err("Empty alias in mapping %s",m.Alias) // Yes, log it.
		} else if m.Pods==""{               // Is the pod name empty?
		  log.Err("Empty pod name in mapping %s",m.Pods) // Yes, log it.
		} else {                            // Else we found something so print it.
		  log.Inf("  Alias: %s -> Pod: %s",m.Alias,m.Pods)// Print the mapping.
		}                                   // Done checking the mapping.
	}                                     // Done printing the mappings.
	log.Inf("HTTP Server object printed.")// Print the HTTP server object.
}                                       // ---------- PrintObject ----------- //
