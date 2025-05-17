/**
* file: main.go
* Description: This program creates multiple child processes (one for each cmd
* line argument), each of which is intended to accomplish some action, simulated
* in the example program by sleeping some time. The parent waits until all
* children have completed their actions. To perform synchronization, the parent
* builds a pipe (1) before creating the child process (2). Each child inherits
* a file descriptor for the write end of the pipe and close the file descriptor
* once it has purpose (3). After all of the children have closed their write
* end file descriptors, the parent's read() (5) from the pipe will complete,
* returning EOF (or 0 bytes read). At this point the parent is free to carry on
* to do other work. (Note that closing the unused write end of the pipe in the
* parent is essential to the correct operation of the technique; otherwise, the
* parent would block forever when trying to read from the pipe.)
*
* Author:
*  J.EP, J. Enrique Peraza
* Reference: The Linux Programming Interface, Michael Kerrisk
*/
package main
import (
  "os"
  "fmt"
  "syscall"
  "strconv"
  "time"
  "io"
  "context"
  "github.com/perazaharmonics/gosys/internal/utils"
  "github.com/perazaharmonics/gosys/internal/logger"
  "github.com/perazaharmonics/gosys/internal/pipe"
)
const BUF_SIZE=10                       // Buffer size for reading from the pipeS

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
func pipeToChildSynch(stdout *os.File,buf []byte, log logger.Log) (int){
 // ----------------------------------- //
 // Create a pipe for synchronization between parent and child processes (1)
 // ----------------------------------- //
 status:=Success                        // Set the status code 
 pfp,err:=pipe.NewPipe()                // Create a new pipe
  if err!=nil{                          // Pipe creation error?
    log.Err("Error creating pipe: %v",err) // Yes, log the error
	status:=PipeError                   // Set the status code
	return status                       // Return the status code
  }                                     // Done checking for pipe creation error.
  log.Inf("Pipe created successfully")  // Log the pipe creation
  defer pfp.Close()                     // Close the pipe when done 
  // ---------------------------------- //
  // Loop for the number of command line arguments - that's
  // the amount of children we want to create. (2)
  // ---------------------------------- //
  for i:=1;i<len(os.Args);i++{          // For the number of cmd line args.
    pid,_,errno:=syscall.RawSyscall(syscall.SYS_FORK,0,0,0) // Fork the process
	if errno!=0{                        // Error forking to new process?
      log.Err("Error forking process: %v",errno) // Yes, log the error
	  status:=ForkError                 // Set the status code
	  return status                     // Return the status code
	}                                   // Done checking for fork error.
	switch pid{                         // Act according to process ID.
	case 0:                             // We are in the child process.
	  log.Inf("Child process %d created",i) // Log the child process creation
      pfp.CloseRead()                   // Close the read end of the pipe
  // ---------------------------------- //
  // Child does some work (simulated by sleeping for a while) and then lets
  // the parent know that it is done by closing the write end of the pipe (3)
  // ---------------------------------- //
      sleepTime,err:=strconv.Atoi(os.Args[i])// Get the sleep time from the cmd line arg
	  if err!=nil{                      // Error converting sleep time to int?
        log.Err("Error converting sleep time to int: %v",err) // Yes, log the error
		status:=UnknownError            // Set the status code
		return status                   // Return the status code
	  }                                 // Done checking for sleep time conversion error.
	  log.Inf("Child process %d sleeping for %d seconds",i,sleepTime) // Log the sleep time
	  time.Sleep(time.Duration(sleepTime)*time.Second) // Sleep for the specified time
	  log.Inf("Child process %d done sleeping",i) // Log the sleep done
	  fmt.Fprintf(stdout,"Child process %d with pid %d done sleeping\n",i,os.Getpid()) // Print the sleep done
	  pfp.CloseWrite()                  // Close the write end of the pipe. (3)
	  return status                     // Return the status code
	default:                            // We are in the parent process.
  // ---------------------------------- //
  // Parent loops to create new child processes. (2)
  // ---------------------------------- //  
	  log.Inf("Handling parent process with pid=%d",pid) // Log the process handling
    }                                   // Done acting according to process ID.
  }                                     // Done with for creating child processes.
  // ---------------------------------- //
  // Parent continues here to close write end of pipe so we can see EOF (4)
  // ---------------------------------- //
  pfp.CloseWrite()                      // Close the write end of the pipe
  log.Inf("Parent process closing write end of pipe") // Log the write end close
  // ---------------------------------- //
  // Parent may do other work, then synchronizes with children by reading from
  // the pipe and checking if its EOF (5)
  // ---------------------------------- //
  log.Inf("Parent process reading from pipe") // Log the read from pipe
  n,err:=pfp.Read(buf)                  // Read from the pipe
  if err!=nil{                          // Error reading from the pipe?
	if err==io.EOF||n==0{               // Yes but it was an EOF?
      log.Inf("EOF encountered")        // Yes, log the EOF
	  status=GotEOF                    // Set the status code
	}                                   // Done checking for EOF.
	log.Err("Error reading from pipe: %v",err) // Yes, log the error
	status:=PipeReadError               // Set the status code
	return status                       // Return the status code
  }                                     // Done checking for read error.
  log.Inf("Completed successfully, status: %s",StatusToString(status))
  return status                         // Return the status code
}                                       // ------------ pipeToChildSynch --------- //

func main(){
  if len(os.Args) < 2 || os.Args[1] == "--help" { // User asking for help?
	fmt.Printf("Usage: %s <sleep-time>\n",os.Args[0]) // Print usage message
	os.Exit(1)                          // Yes exit program.
  }                                     // Done checking for help.
  log,err:=logger.NewLogger()           // Create a new logger
  if err!=nil{                          // Error creating logger?
    fmt.Fprintf(os.Stderr,"error creating logger: %v\n",err)
    os.Exit(1)                          // Yes, exit program.
  }                                     // Done creating logger object.
  // ---------------------------------- //
  // Make stdout unbuffered so we can see the output immediately.
  // ---------------------------------- //
  stdout:=os.Stdout                     // Get the stdout file descriptor 
  stdout.Sync()                         // Synchronize stdout.
  stdout=os.NewFile(uintptr(syscall.Stdout),stdout.Name()) // Create a new file descriptor for stdout
  // ----------------------------------- //
  // No matter how we exit the program we need to close the logger.
  // So we can clean the semaphore.
  // ----------------------------------- //
  _,cancel:=context.WithCancel(context.Background()) // Create a context						            
  utils.SignalHandler(cancel)		    // Set up signal handler
  utils.RegisterShutdownCB(func(){      // Register shutdown callback
    log.Inf("Shutdown callback called.")
    log.Shutdown()                      // Shutdown the logger
  })                                    // Done registering shutdown callback
  buf:=make([]byte,BUF_SIZE)            // Create a buffer for reading from the pipe
  utils.SetLogger(log)				    // Set the logger object
  status:=pipeToChildSynch(stdout,buf,log)// Call the pipeToChild function
  if status!=Success&&status!=GotEOF{   // Report the error
    log.Err("Pipe to child process state returned error: %s",StatusToString(status))
  } else{                               // The good ending.
    log.Inf("Pipe to child process state returned: %s",StatusToString(status))
  }                                     // Done logging.
 cancel()                               // Send context cancellation signal.
 log.Inf("Program exited.")             // Log goodbye.
 utils.InvokeShutdownCBs()              // Nice cleanup.
}