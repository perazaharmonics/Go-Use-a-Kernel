package httpserver
import (
  "net/http"
	"net/http/httptest"
	"testing"
	"strings"
	"time"
	"os/exec"                          // For exec.Command
	"context"                         // For context management
)

const cfgpath="/home/ljt/Projects/NetGo/project_name/TestMapping.yaml"
const port=8080
// Helper function that just does string compare
func strCmp(str1, str2 string) bool {
  return strings.Contains(str1,str2)
}

// ------------------------------------- //
// Test the HTTP server initializer.
// ------------------------------------- //
func TestNewHttpServer(t *testing.T) {
  // Create the HTTP server object.
	mport:=9090                            // Mock port number.
	srv:=NewHttpServer(mport,cfgpath,"")   // Init the server object.
	// ----------------------------------- //
	// Test if we were able to create a new server object with these values.
	// ----------------------------------- //
		if srv.port != mport {
			t.Errorf("Expected port %d to be assigned to server but got %d", port, srv.port)
		}
		if srv.cfgp != cfgpath {
			t.Errorf("Expected config path %s to be assigned to server but got %s", cfgpath, srv.cfgp)
		}
		if srv.vrs!="1.1.0d" {
			t.Errorf("Expected version %s to be obtained from the Proxy.vdd but got %s", "1.1.0d", srv.vrs)
		}
		if srv.now.IsZero() {
			t.Errorf("Expected current time to be set but got zero value")
		}
}                                       // ------ TestNewHttpServer --------- //
// ------------------------------------ //
// Test the LoadMappings function.
// ------------------------------------ //
func TestLoadMappings(t *testing.T) {
  s:=NewHttpServer(port,cfgpath,"")     // Create a new HTTP server instance.
	empty:=false                          // Set flag if no mappings are found.
	if err:=s.LoadMappings(); err!=nil {  // Could we load the config?
	  t.Errorf("Could not load config file %s: %v",cfgpath,err)
		return                              // No, return with an error.
	}                                     // Done loading the config file.
	if len(s.maps)==0 {                   // Did we get any mappings?
	  t.Errorf("Expected mappings to be loaded but got none")
		empty=true                          // Set the empty flag.
	}                                     // Done checking for mappings.
	for _, m := range s.maps {            // Iterate over the mappings.
	  if m.Alias == "" {                  // Is the alias empty?
		  t.Errorf("Expected alias to be set but got empty string")
		  empty=true                        // Set the empty flag.
		}                                   // Done checking for emtpy alias.
		if m.Pods == "" {                   // Is the pod name empty?
		  t.Errorf("Expected pod name to be set but got empty string")
		  empty=true                        // Set the empty flag.
		}                                   // Done checking for empty pod name.
	  if empty {                          // Did we find any empty values?
		  t.Errorf("Expected mappings to be valid but got empty values")
			return                            // Yes, return with an error.
		} else {                            // Else, we found something.
		  t.Logf("Found mapping: %s -> %s", m.Alias, m.Pods) // Log the mapping.
		}                                   // Done logging the mapping.                                  
	}                                     // Done iterating over the mappings.
}                                       // ------- TestLoadMappings --------- //
// ------------------------------------ //
// Test the atomic connection counters
// ------------------------------------ //
func TestConnectionCounter (t *testing.T) {
	s:=NewHttpServer(8080,cfgpath,"")   // Create a new HTTP server instance.
	fail:=false                         // Set the control flag.
	s.IncrementConnections()            // Increment the connection counter.
	if s.GetConnections()!=1 {          // Could we increment the counter?
	  t.Errorf("Expected connection counter to be 1 but got %d",s.GetConnections())
		fail=true                         // No, set the failure flag.
	} else {                            // Else we increment so let's decrement it.
	  s.DecrementConnections()          // Decrement the connection counter.
	    if s.GetConnections()!=0 {      // Could we decrement the counter?
			  t.Errorf("Expected connection counter to be 0 but got %d",s.GetConnections())
				fail=true                     // No, set the failure flag.
			}                               // Done with could not decrement counter.
	}                                   // Done with could increment counter.
	if fail {                           // Did we fail somewhere?
	  t.Errorf("Expected connection counter to be 0 but got %d",s.GetConnections())
		return                            // Yes, return with an error.
	} else {                            // Else no errors occured.
	  t.Logf("Connection counter: %d",s.GetConnections()) // Log the connection counter.
	}                                   // Done logging the connection counter.
}                                     // ------ TestConnectionCounter ------- //
// ---------------------------------- //
// Test the atomic cache hit/miss counters
// ---------------------------------- //
func TestCacheCounter (t *testing.T) {
  s:=NewHttpServer(port,cfgpath,"")   // Create a new HTTP server instance.
	fail:=false
	s.IncrementCacheHits()              // Increment the cache hit counter.
	if s.GetCacheHits()!=1 {            // Could we increment the counter?
	  t.Errorf("Expected cache hit counter to be 1 but got %d",s.GetCacheHits())
		fail=true                         // No, set the failure flag.
	}                                   // Done with could not increment cache hits.
	s.IncrementCacheMisses()            // Increment the cache miss counter.
	if s.GetCacheMisses()!=1 {          // Could we increment the counter?
	  t.Errorf("Expected cache miss counter to be 1 but got %d",s.GetCacheMisses())
		fail=true                         // No, set the failure flag.
	}                                   // Done with could not increment cache misses.
	if fail {                           // Did we get some error?
	  t.Errorf("Expected cache hits/misses to be 1 but got %d",s.GetCacheHits())
		return                            // Yes, return with an error.
	} else {                            // Else no errors occured.
	  t.Logf("Cache hits: %d",s.GetCacheHits()) // Log the cache hits.
		t.Logf("Cache misses: %d",s.GetCacheMisses()) // Log the cache misses.
	}                                   // Done logging the cache hits/misses.
}                                     // ------ TestCacheCounter ------------ //
// ---------------------------------- //
// Test the readiness probe response logic.
// ---------------------------------- //
func TestReadinessProbe(t *testing.T) {
  s:=NewHttpServer(port,cfgpath,"")   // Create a new HTTP server instance.
	// -------------------------------- //
	// We will have to simulate that we have been "up"" for 20 seconds such that
	// the readiness gate will open (the time trigger for this event is 20s default
	// .... that might be a bit too long, but for now we will leave it like that).
	// -------------------------------- //
	s.now=time.Now().Add(-30*time.Second)// Simulate we've met the readiness gate.
	s.rpwt=20*time.Second               // Set to the default to test
	req:=httptest.NewRequest("GET","/readyz",nil)// Create a GET /readyz request.
	w:=httptest.NewRecorder()           // Create a new response recorder.
	fail:=false                         // Set the control flag
	s.ReadinessProbe(w,req)             // Call the readiness probe hadnler.
	resp:=w.Result()                    // Get the response from the recorder.
	body:=w.Body.String()               // Get the response body.
	if resp.StatusCode!=http.StatusOK { // Did we get a 200 OK response?
	  t.Errorf("Expected status code 200 but got %d",resp.StatusCode)
		fail=true                         // No, set the failure flag.
	}                                   // Done with checking NOT OK response.
	if body != "Ready" {                // Did we get the expected response body?
	  t.Errorf("Expected response body to be 'Ready' but got %s",body)
		fail=true                         // No, set the failure flag.
	}                                   // Done with checking bad response body.
  if fail {                           // Did we get an error?
	  t.Errorf("Expected readiness to be Ready but got %s",body)
		return                            // Yes, return with an error.
	} else {                            // Else no errors ocurred.
	  t.Logf("Response code: %d",resp.StatusCode)// Log the response Status code.
	  t.Logf("Response body: %s", body) // If we got here we are good.	
	}                                   // Done handling some failure.
}                                     // ------ TestReadinessProbe --------- //
// ---------------------------------- //
// Test the liveness probe response logic.
// ---------------------------------- //
func TestLivenessProbe(t *testing.T) {
  s:=NewHttpServer(port,cfgpath,"")     // Create a new http server instance.
	s.now=time.Now().Add(-10*time.Second) // Simulate we've been up for 10 seconds.
	s.lpwt=5*time.Second                  // Set liveness probe wait time to 5s.
	req:=httptest.NewRequest("GET","/healthz",nil)// Create a GET /healthz request.
	w:=httptest.NewRecorder()             // Create a new response recorder.
	fail:=false                           // Set control flag.
	s.LivenessProbe(w,req)                // Call the liveness probe handler.
	resp:=w.Result()                      // Get the response from the recorder.
	body:=w.Body.String()                 // Get the response body.
	if resp.StatusCode!=http.StatusOK {   // Did we get a 200 OK response?
	  t.Errorf("Expected status code 200 but got %d",resp.StatusCode)
		fail=true                           // No, set the failure flag.
	}                                     // Done with checking NOT OK response.
	if body != "OK" {                     // Did we get the expected response body?
	  t.Errorf("Expected response body to be 'OK' but got %s",body)
		fail=true                           // No, set the failure flag.
	}                                     // Done with checking bad response body.
  if fail {                             // Did we get an error?
	  t.Errorf("Expected liveness to be 'OK' but got %s",body)
		return                              // Yes, return with an error.
	} else {
	  t.Logf("Response code: %d",resp.StatusCode)// Log the response Status code.
	  t.Logf("Response body: %s", body)     // If we got here we are good.	  
	}
}                                       // ------ TestLivenessProbe ---------- //
// ------------------------------------ //
// Test the ping probe response logic.
// ------------------------------------ //
func TestPingProbe(t *testing.T) {
  s:=NewHttpServer(port,cfgpath,"")     // Create a new http server instance.
	req:=httptest.NewRequest("GET","/pingz",nil)// Create a GET /pingz request.
	fail:=false                           // Set control flag.
	w:=httptest.NewRecorder()             // Create a new response recorder.
	s.PingProbe(w,req)                    // Call the ping probe handler.
	resp:=w.Result()                      // Get the response from the recorder.
	body:=w.Body.String()                 // Get the response body.
	if resp.StatusCode!=http.StatusOK {   // Did we get a 200 OK response?
	  t.Errorf("Expected status code 200 but got %d",resp.StatusCode)
		fail=true                           // No, set the failure flag.
	}                                     // Done with checking NOT OK response.
	if body != "pong" {                   // Did we get the expected response body?
	  t.Errorf("Expected response body to be 'pong' but got %s",body)
		fail=true                           // No, set the failure flag.
	}                                     // Done with checking bad response body.
  if fail {                             // Did we get an error?
	  t.Errorf("Expected ping to be pong but got %s",body)
	} else {                              // Else no errors occured.
	  t.Logf("Response code: %d",resp.StatusCode)// Log the response Status code.
	  t.Logf("Response body: %s", body)   // If we got here we are good.	
	}                                     // Done with checking the response body.
}                                       // ---------- TestPingProbe --------- // 
// ------------------------------------ //
// Test the version probe response logic.
// ------------------------------------ //
func TestVersionProbe(t *testing.T) {
	s:=NewHttpServer(port,cfgpath,"")     // Create a new http server instance.
	req:=httptest.NewRequest("GET","/versionz",nil)// Create a GET /versionz request.
	fail:=false                            // Set control flag.
	w:=httptest.NewRecorder()             // Create a new response recorder.
	s.VersionProbe(w,req)                 // Call the version probe handler.
	resp:=w.Result()                      // Get the response from the recorder.
	body:=w.Body.String()                 // Get the response body.
	if resp.StatusCode!=http.StatusOK {   // Did we get a 200 OK response?
	  t.Errorf("Expected status code 200 but got %d",resp.StatusCode)
		fail=true                           // No, set the failure flag.	
	}                                     // Done with checking NOT OK response.
	if body != "Proxy Version: 1.1.0d\n" {        // Did we get the expected response body?
	  t.Errorf("Expected response body to be 'Proxy Version: 1.1.0d' but got %s",body)
		fail=true                           // No, set the failure flag.
	}                                     // Done with checking bad response body.
  if fail {                             // Did we get an error?
	  t.Errorf("Expected version to be 1.1.0d but got %s",body)
		return                              // Yes, return with an error.
	} else {                              // Else no errors occured.
	  t.Logf("Response code: %d",resp.StatusCode)// Log the response Status code.
		t.Logf("Response body: %s", body)   // If we got here we are good.
	}                                     // Done with checking the response body.
}                                       // ------ TestVersionProbe ----------- //
// ------------------------------------ //
// Test the status probe response logic.
// ------------------------------------ //
func TestStatusProbe(t *testing.T) {
	s:=NewHttpServer(port,cfgpath,"")     // Create a new http server instance.
  s.LoadMappings()                      // Load the mappings from the config file.
	fail:=false                           // Set control flag.
	req:=httptest.NewRequest("GET","/statusz",nil)// Create a GET /statusz request.
	w:=httptest.NewRecorder()             // Create a new response recorder.
	s.isready=true                        // Set the server to ready.
	s.StatusProbe(w,req)                  // Call the status probe handler.
	resp:=w.Result()                      // Get the response from the recorder.
	body:=w.Body.String()                 // Get the response body.
	if resp.StatusCode!=http.StatusOK {   // Did we get a 200 OK response?
	  t.Errorf("Expected status code 200 but got %d",resp.StatusCode)
		fail=true                           // Set the failure flag.
	}                                     // Done with checking NOT OK response.
	if !strCmp(body,"Proxyd up since") {
	  t.Errorf("Expected response body to contain 'Proxyd up since' but got %s",body)
		fail=true                           // Set the failure flag.
	}                                     // Done with checking bad response body.
	if fail {                             // Did we get an error?
	  t.Errorf("Expected status to be OK but got %d with body %s",resp.StatusCode,body)
		return                              // Yes, return with an error.
	} else {                              // Else no errors occured.
	  t.Logf("Response code: %d",resp.StatusCode)// Log the response Status code.
	  t.Logf("Response body: %s", body)   // If we got here we are good.	
	}                                     // Done with checking the response body.
}                                       // --------- TestStatusProbe -------- //
// ------------------------------------ //
// Test the map probe response logic.
// ------------------------------------ //
func TestMetricProbe(t *testing.T) {
  s:=NewHttpServer(port,cfgpath,"")     // Create a new http server instance.
  fail:=false                           // Set control flag.
	s.now=time.Now().Add(-60*time.Second) // Simulate we've been up for 60s.
	s.IncrementConnections()              // Increment the connection counter.
	s.IncrementCacheHits()                // Increment the cache hit counter.
	s.IncrementCacheMisses()              // Increment the cache miss counter.
	s.IncrementReloads()                  // Increment the reload counter.
	req:=httptest.NewRequest("GET","/metricz",nil)// Create a GET /mapz request.
	w:=httptest.NewRecorder()             // Create a new response recorder.
	s.MetricProbe(w,req)                  // Call the map probe handler.
	resp:=w.Result()                      // Get the response from the recorder.
	body:=w.Body.String()                 // Get the response body.
	if resp.StatusCode!=http.StatusOK {   // Did we get a 200 OK response?
	  t.Errorf("Expected status code 200 but got %d",resp.StatusCode)
		fail=true                           // Set the failure flag.
	}                                     // Done with checking NOT OK response.
	if !strCmp(body, "proxyd_uptime_second") || !strCmp(body,"proxyd_connections_total") ||
	   !strCmp(body,"proxyd_cache_hits") {
	  t.Errorf("Missing expected Prometheus metrics in response body.")
		fail=true
	}                                     // Done with checking bad response body.
	if fail {                             // Did we get any errors?
	  t.Errorf("Expected metrics to be loaded but got none")
		return                              // Yes, return with an error.
	} else {                              // Else no errors occured.
	  t.Logf("Response code: %d",resp.StatusCode)// Log the response Status code.
	  t.Logf("Response body: %s", body)   // If we got here we are good.	
	}                                     // Done with checking the response body.
}                                       // -------- TestMetricProbe --------- //
// ------------------------------------ //
// Test the map probe response logic.
// ------------------------------------ //
func TestMapProbe(t *testing.T) {
  s:=NewHttpServer(port,cfgpath,"")     // Create a new http server instance.
	fail:=false                           // Set control flag.       
	if err:=s.LoadMappings(); err!=nil {  // Could we load the config?
	  t.Errorf("Could not load config file %s: %v",cfgpath,err)
	  fail=true                           // Set the failure flag.
	}                                     // Done loading the config file.
	req:=httptest.NewRequest("GET","/mapz",nil)// Create a GET /mapz request.
	w:=httptest.NewRecorder()             // Create a new response recorder.
	s.MapProbe(w,req)                     // Call the map probe handler.
	resp:=w.Result()                      // Get the response from the recorder.
	body:=w.Body.String()                 // Get the response body.
	if resp.StatusCode!=http.StatusOK {   // Did we get a 200 OK response?
	  t.Errorf("Expected status code 200 but got %d",resp.StatusCode)
		fail=true                           // Set the failure flag.
	}                                     // Done with checking NOT OK response.
	if !strCmp(body,"test-pod") {
	  t.Errorf("Missing expected mappings in response body.")
		fail=true                           // Set the failure flag.	 
	}                                     // Done with checking bad response body.
	if fail {                             // Did we find any errors?
	  t.Errorf("Expected mappings to be loaded but got none")
		return                              // Yes, return with an error.
	} else {                              // Else no errors occured.
	  t.Logf("Response code: %d",resp.StatusCode)// Log the response Status code.
		t.Logf("Response body: %s", body)   // If we got here we are good.
	}                                     // Done with checking the response body.
}                                       // --------- TestMapProbe ----------- //
// ------------------------------------ //
// Test the reload probe response logic.
// ------------------------------------ //
func TestReloadProbe (t *testing.T) {
  s:=NewHttpServer(port,cfgpath,"")     // Create a new http server instance.
	fail:=false                           // Set the control flag.
	req:=httptest.NewRequest("GET","/reloadz",nil)// Create a GET /reloadz request.
	w:=httptest.NewRecorder()             // Create a new response recorder.
	s.ReloadProbe(w,req)                  // Call the reload probe handler.
	resp:=w.Result()                      // Get the response from the recorder.
	body:=w.Body.String()                 // Get the response body.
  if resp.StatusCode!=http.StatusAccepted {// Did we receive expected response?
	  t.Errorf("Expected status code 202 but got %d",resp.StatusCode)
		fail=true                           // Set the failure flag.
	}                                     // Done with did not receive Accepted code.
	if !strCmp(body,"Reload initiated") { // Did the body match the expected value?
	  t.Errorf("Expected response body to be 'Reload initiated' but got %s",body)
		fail=true                           // No, set the failure flag.
	}                                     // Done with body did not match.
	if fail {                             // Did we find any error?
	  t.Errorf("Expected a configuration reload, but failed to do so.")
		return                              // Yes, return with the error.
	} else {                              // Else no errors ocurred.
	  t.Logf("Response code: %d",resp.StatusCode)// Log the response Status code.
		t.Logf("Response body: %s", body)   // If we got here we are good.
	}                                     // Done printing response.                                                                      
}                                       // ------- TestReloadProbe --------- //
// ------------------------------------ //
// Test the rotate logs probe response logic.
// ------------------------------------ //
func TestRotateLogs(t *testing.T) {
  // ---------------------------------- //
	// We will have to simulate that we have a script to run so that the 
	// unit test does not shell out into a job
	// ---------------------------------- //
	origExec:=execCommandContext          // Save the original execCommandContext function.
	execCommandContext = func(ctx context.Context, name string, arg...string) *exec.Cmd {
	  return exec.Command("true")
	}
	defer func() {execCommandContext=origExec}() // Restore the original function.
	s:=NewHttpServer(port,cfgpath,"")     // Create a new http server instance.
	// ---------------------------------- //
	// Now we can point the server to some throwaway script to run.
	// ---------------------------------- //
	s.SetRotateScript("/some/throwaway/script.sh") // Set the script to run.
	fail:=false                           // Set the control flag.
	req:=httptest.NewRequest("GET","/rotateLogs",nil)// Create a GET /reloadz request.
	w:=httptest.NewRecorder()             // Create a new response recorder.
	s.RotateLogs(w,req)                   // Call the reload probe handler.
	resp:=w.Result()                      // Get the response from the recorder.
	body:=w.Body.String()                 // Get the response body.
  if resp.StatusCode!=http.StatusAccepted {// Did we receive expected response?
	  t.Errorf("Expected status code 202 but got %d",resp.StatusCode)
		fail=true                           // Set the failure flag.
	}                                     // Done with did not receive Accepted code.
	if !strCmp(body,"Log rotation successful") { // Did the body match the expected value?
	  t.Errorf("Expected response body to be 'Log rotation successful' but got %s",body)
		fail=true                           // No, set the failure flag.
	}                                     // Done with body did not match.
	if fail {                             // Did we find any error?
	  t.Errorf("Expected a log rotation, but failed to do so.")
		return                              // Yes, return with the error.
	} else {                              // Else no errors ocurred.
	  t.Logf("Response code: %d",resp.StatusCode)// Log the response Status code.
		t.Logf("Response body: %s", body)   // If we got here we are good.
	}                                     // Done printing response.   		
}                                       // ------- TestRotateLogs --------- //
   