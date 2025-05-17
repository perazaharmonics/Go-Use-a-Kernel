/**
* file: main.go
* Description: This program demonstrates the use of the popen() syscall and pclose().
* The program repeatedly reads a filename wildcard pattern (2), and then uses popen()
* to obtain the results from passing this pattern to the ls cmd. (5). Techniques
* similar to these we used on older UNIX implementations to perform filename
* generation, also known as globbing, prior to the existence of the glob() syscall.
* The construction of the cmd (1)(4) for globbing, and input checking is done to
* ensure that no invalid input is passed to POpen().
* Author:
*  J.EP, J. Enrique Peraza
* Reference: The Linux Programming Interface, Michael Kerrisk
*/
package main
import (
  "os"
  "fmt"
  "io"
  "context"
  "bufio"
  "github.com/perazaharmonics/gosys/internal/utils"
  "github.com/perazaharmonics/gosys/internal/logger"
  "github.com/perazaharmonics/gosys/internal/pipe"
)
// (1) 
const(
	POPEN_FMT="/bin/ls -d %s 2> /dev/null"
	PAT_SIZ=50
	PCMD_BUF_SIZ=len(POPEN_FMT)+PAT_SIZ
)

const (
	Success=iota                        // No errors
	ForkError						    // Fork error
	PipeError                           // Pipe error
	PipeCreated                         // Pipe created successfully
	PipeReadEndClosed                   // Read end of pipe closed
	PipeWriteEndClosed                  // Write end of pipe closed
	PipeReadError                       // Read error
	PipeWriteError                      // Write error
	GotEOF                              // EOF encountered
	UnknownError                        // Unknown error
)
func StatusToString(status int) string {// Convert status code to string
	switch status {                     // Check the status code
	case ForkError:                     // Fork error
		return "Fork error"             // Return the string
	case PipeError:                     // Pipe error
		return "Pipe error"             // Return the string
	case Success:                       // No errors
		return "Success"                // Return the string
	case PipeCreated:                   // Pipe created successfully
		return "Pipe created successfully"// Return the string
	case PipeReadEndClosed:             // Read end of pipe closed
		return "Read end of pipe closed"// Return the string
	case PipeWriteEndClosed:            // Write end of pipe closed
		return "Write end of pipe closed"// Return the string
	case PipeReadError:                 // Read error
		return "Read error"             // Return the string
	case PipeWriteError:                // Write error
		return "Write error"            // Return the string
  case GotEOF:                          // EOF encountered
		return "EOF encountered"        // Return the string
	case UnknownError:                  // Unknown error
		return "Unknown error"          // Return the string
	default:                            // Unknown status code
		return "Unknown status code"    // Return the string
	}                                   // Done stringing the status code.
}                                       // ------------ StatusToString --------- //
func pipeFromShell(pat []byte,log logger.Log) (int,error){
  // Read pattern and display result from globbing.
 status:=Success                        // Status flag.
 badpat:=false                          // Bad pattern flag.
 for{                                  // Loop until EOF.
	writer := bufio.NewWriter(os.Stdout)
	writer.Flush()                      // Flush stdout.
	reader:=bufio.NewReader(os.Stdin)   // Create a new reader.
	line,err:=reader.ReadString('\n')   // Read until we encounter a newline.
	if err!=nil{                        // Check for errors.
	  if err==io.EOF{                   // EOF encountered?
		log.Inf("EOF encountered")      // Yes, print message.
		status=GotEOF                   // Yes, set status to EOF.
		break                           // Yes exit loop.
	  }                                 // Done checking for EOF.
	  if len(line)==0{                  // Empty line?
        continue                        // Yes, continue to next iteration.
	  }                                 // Done checking for empty line.
	  log.Err("Error reading input: %v",err) // Print error message.
      status=UnknownError               // Set status to unknown error.
	  return status,err                 // Return status and error.
	}                                   // Done checking for read err.
	line=line[:len(line)-1]             // Remove newline from line.
	if len(line)>PAT_SIZ{
	  log.Err("Pattern too long: %s",line) // Print error message.
	  status=UnknownError               // Set status to unknown error.
	  return status,fmt.Errorf("pattern too long")
	}
	copy(pat,line)                      // Copy line to pattern buffer.
	log.Inf("Processing pattern: %s",line) // Print pattern.
  
	// -------------------------------- //
	// Ensure that the pattern contains only valid characters. (3)
	// We allow only alphanumeric characters, underscores, dashes, slashes,
	// and the shell globbing patterns '*', '?', and '['.
	// -------------------------------- //
	for j := 0; j < len(line) && !badpat; j++ {
	  if line[j] == '*' || line[j] == '?' || line[j] == '[' {
		continue					    // Yes, continue to next iteration.
	  } else if line[j] == '/' {        // Is it a slash?
		continue					    // Yes, continue to next iteration.
	  } else if line[j] == '_' {        // Is it an underscore?
		continue					    // Yes, continue to next iteration.
	  } else if line[j] == '-' {        // Is it a dash?
		continue					    // Yes, continue to next iteration.
	  } else if line[j] >= '0' && line[j] <= '9' { // Is it a digit?
		continue					    // Yes, continue to next iteration.
	  } else if line[j] >= 'A' && line[j] <= 'Z' { // Is it an uppercase letter?
		continue					    // Yes, continue to next iteration.
	  } else if line[j] >= 'a' && line[j] <= 'z' { // Is it a lowercase letter?
		continue					    // Yes, continue to next iteration.
	  } else {                          // No, it is not a valid character.
		badpat = true                   // Set bad pattern flag.
		log.Err("Invalid pattern: %s", line) // Print error message.
	  }                                 // Done checking for valid characters.
	}
	// -------------------------------- //
	// Build and execute command to glob pattern. Then read from pipe 
	// -------------------------------- //
	cmd:=fmt.Sprintf(POPEN_FMT,string(line)) // Build command.
	log.Inf("Executing command: %s",cmd) // Print command.
	fd,pid,err:=pipe.Popen(cmd,pipe.POPENREAD) // Execute command.
	if err!=nil{                         // Check for errors.
	  log.Err("Error executing command: %v",err) // Print error message.
	  status=PipeError                  // Set status to pipe error.
	  return status,err                 // Return status and error.
	}                                   // Done checking for errors.
	f:=os.NewFile(uintptr(fd),"popen-r")
	if f==nil{                          // Check for errors.
	  log.Err("Error creating file from fd: %v",err) // Print error message.
	  status=PipeError                  // Set status to pipe error.
	  return status,fmt.Errorf("error creating file from fd")
	}                                   // Done checking for errors.
	proc,err:=os.FindProcess(pid)       // Find process by pid.
	if err!=nil{                        // Error finding proc?
      log.Err("Error finding process with pid %d: %v",pid,err) // Print error message.
	 f.Close()                          // Close file. 
	  status=UnknownError               // Set status to unknown error.
	  return status,fmt.Errorf("error finding process with pid %d",pid)
	}                                   // Done checking for errors.
	// -------------------------------- //
	// Read from pipe and display results.resulting list of pathanmes
	// until EOF.
	// -------------------------------- //
	fgets:=func(fd,maxSiz int)(string,error){
	  f:=os.NewFile(uintptr(fd),"pipe") // Create a new file from fd.
	  if f==nil{                        // Check for errors.
        return "",fmt.Errorf("invalid file descriptor")
	  }                                 // Done checking for errors.
	  defer f.Close()                   // Close file when done.
	  reader:=bufio.NewReader(f)        // Create a new reader.
	  line,err:=reader.ReadString('\n') // Read until we encounter a newline.
	  if err!=nil{                      // Check for errors.
		if err==io.EOF{                 // EOF encountered?
		  log.Inf("EOF encountered")    // Yes, print message.
		  status=GotEOF                 // Yes, set status to EOF.
		  return "",io.EOF              // Yes, return empty string and nil.
		}                               // Done checking for EOF.
	  return "",err                     // Return empty string and error.
	}                                   // Done checking for read err.
	line=line[:len(line)-1]             // Remove newline from line.
	if len(line)>maxSiz{                // Is line too long?
	  return "",fmt.Errorf("line too long") // Yes, return error.
	}                                   // Done checking for line length.
	return line,nil                     // Return line and nil.
  }	                                    // Done defining fgets.
  n:=0                                  // Our file counter.
  for{                                  // Loop until EOF.
	line,err:=fgets(fd,PCMD_BUF_SIZ)    // Read from pipe.
	if err!=nil{                        // Error reading from pipe?
      if err==io.EOF{                   // Was it EOF?
        log.Inf("EOF encountered")      // Yes, print message.
		status=GotEOF                   // Yes, set status to EOF.
		break                           // Yes, exit loop.
	  }                                 // Done checking for EOF.
	  log.Err("Result: %s",line)        // Print error message.
	  status=PipeReadError              // Set status to pipe read error.
	  return status,err                 // Return status and error.
	}                                   // Done checking for read err.
	// -------------------------------- //
	// Close pipe, fetch and display results.
	// -------------------------------- //
	  status,_=pipe.PClose(f,proc)    // Close pipe and wait for child process to exit.
	  if status!=0{                       // Check for errors.
	    log.Err("Error closing pipe: %v",status) // Print error message.
	    status=PipeReadEndClosed 
	    return status,fmt.Errorf("error closing pipe")
	  }                                   // Done checking for errors.
	  if len(line)>0{                   // Is line empty?
	    n++	                            // No, we found something.  
	    log.Inf("Result %d: %s",n,line) // Print result.
	    fmt.Println(line)               // Print result.
	  }                                 // Done checking for empty line.
    }                                   // Done reading from pipe.
  }                                     //
  return status,nil                     // Return status and nil.
}                                       // 

func main() {
  log,err:=logger.NewLogger()           // Create a new logger
  if err!=nil{                          // Error creating logger?
    fmt.Fprintf(os.Stderr,"error creating logger: %v\n",err)
    os.Exit(1)                          // Yes, exit program.
  }                                     // Done creating logger object.
  // ---------------------------------- //
  // No matter how we exit the program we need to close the logger.
  // So we can clean the semaphore.
  // ---------------------------------- //
  _,cancel:=context.WithCancel(context.Background()) // Create a context						            
  utils.SignalHandler(cancel)		    // Set up signal handler
  utils.RegisterShutdownCB(func(){      // Register shutdown callback
    log.Inf("Shutdown callback called.")
    log.Shutdown()                      // Shutdown the logger
  })                                    // Done registering shutdown callback
  utils.SetLogger(log)				    // Set the logger object  
  buf:=make([]byte,PAT_SIZ)             // Create a buffer for reading from the pipe
  status,_:=pipeFromShell(buf,log)      // Call the pipeFromShell function
  if status!=Success&&status!=GotEOF{   // Report the error
    log.Err("Pipe to child process state returned error: %s",StatusToString(status))
  } else{                               // The good ending.
    log.Inf("Pipe to child process state returned: %s",StatusToString(status))
  }                                     // Done logging.
 cancel()                               // Send context cancellation signal.
 log.Inf("Program exited.")             // Log goodbye.
 utils.InvokeShutdownCBs()              // Nice cleanup.
}                                       // Done invoking shutdown callbacks.