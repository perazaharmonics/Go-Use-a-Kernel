/**
* filename: main.go
* This program demonstrates the use of a pipe for communication between 
* parent and child processes. It demonstrates the byte-stream nature of pipes
* where the parent writes its data in a single operation, while the child reads
* data from the pipe in small blocks. The main program calls the NewPipe() wrapper
* of the syscall pipe() to create a pipe (1), and then forks a child process to
* create a child process (2). After the fork the parent process closes the fd
* for the read end of the pipe (8), and writes the string given as the programs
* command line argument to the write end of the pipe (9). The parent closes the
* write end of the pipe (10) and waits for the child to terminate (11). The child
* process enters a loop where it reads (4) blocks of data (up to BUF_SIZE bytes) from the
* and writes (6) them to stdout. When the child encounters EOF (5) it exits the loop (7)
* writes a trailing newline character, closes its descriptor for the read end of the
* pipe, and terminates.
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
  "io"
  "context"
  "github.com/perazaharmonics/gosys/internal/utils"
  "github.com/perazaharmonics/gosys/internal/logger"
  "github.com/perazaharmonics/gosys/internal/pipe"
)
const BUF_SIZE=10                       // Buffer size for reading from the pipeS

const (
	Success=iota
	ForkError						    // Fork error
	PipeError
	PipeCreated                         // Pipe created successfully
	PipeReadEndClosed                   // Read end of pipe closed
	PipeWriteEndClosed                  // Write end of pipe closed
	PipeReadError                       // Read error
	PipeWriteError                      // Write error
	GotEOF                              // EOF encountered
	UnknownError                        // Unknown error
)

func StatusToString(status int) string { // Convert status code to string
	switch status {                     // Check the status code
	case ForkError:                     // Fork error
		return "Fork error"             // Return the string
	case PipeError:                     // Pipe error
		return "Pipe error"             // Return the string
	case Success:                       // No errors
		return "Success"                // Return the string
	case PipeCreated:                   // Pipe created successfully
		return "Pipe created successfully" // Return the string
	case PipeReadEndClosed:             // Read end of pipe closed
		return "Read end of pipe closed" // Return the string
	case PipeWriteEndClosed:            // Write end of pipe closed
		return "Write end of pipe closed"// Return the string
	case PipeReadError:                 // Read error
		return "Read error"             // Return the string
	case PipeWriteError:                // Write error
		return "Write error"            // Return the string
		case GotEOF:                    // EOF encountered
		return "EOF encountered"        // Return the string
	case UnknownError:                  // Unknown error
		return "Unknown error"          // Return the string
	default:                            // Unknown status code
		return "Unknown status code"    // Return the string
	}                                   // Done stringing the status code.
}                                       // ------------ StatusToString --------- //

func pipeToChild(buf []byte, log logger.Log) (int){
  status:=Success                       // Initialize status to Success
  // ---------------------------------- //
  // Attempt to create a new pipe (1).
  // ---------------------------------- // 
  p,err:=pipe.NewPipe()		            // Call the pipe wrapper to create a pipe
  if err!=nil{                          // Did we error initializing the pipe?
    log.Err("Error creating pipe: %v",err) // Yes, return nil object and error.
	status=PipeError                    // Set status to PipeError
	return status                       // Yes, signal error.
  }                                     // Done with error creating pipe.
  log.Inf("Pipe created successfully.") // Pipe created successfully
  defer p.Close()                       // Defer closing the pipe
  // ---------------------------------- // 
  // Fork to create a child process (2).
  // ---------------------------------- //
  pid,_,errno:=syscall.RawSyscall(syscall.SYS_FORK,0,0,0) // Fork the process
  if errno!=0{                         // Did we error forking the process?
	log.Err("Error forking process: %v",errno) // Yes, return nil object and error.
	status=ForkError                   // Set status to ForkError
	return status                      // Yes, signal error.                           
  }                                    // Done with error forking process.
  switch pid{                          // Act according to the pid.
  case 0:                              // We are in the child process
    log.Inf("Child process created.")  // Child process created
	// ------------------------------- //
	// We are the child so we will be reading from the pipe.
	// ------------------------------- //
	re,err:=p.GetReadEnd()             // Get the write end of the pipe
	if err!=nil{                       // Did we error getting the write end of the pipe?
		log.Err("Error getting write end of pipe: %v",err)
		status=PipeReadEndClosed       // Set status to PipeReadEndClosed
		return status                  // Yes, signal error.
	}                                  // Done checking for error getting write end of pipe.
	p.CloseWrite()                     // Close the write end of the pipe
	// ------------------------------- //
	// Now we read data from the pipe and echo on stdout.
	// ------------------------------- //
	for{                               // Loop until EOF
		numRead,err:=re.Read(buf)      // Read from the pipe
		if err!=nil{                   // Did we error reading from the pipe?
		  if err==io.EOF||numRead==0{  // Yes, did we get EOF? (5)
            log.Inf("EOF encountered.") // Yes, log EOF
			status=GotEOF              // Set status to GotEOF
			break                      // Break out of the loop
		  }                            // Done checking for EOF.
		  log.Err("Error reading from pipe: %v",err) // Yes, return nil object and error.
		  status=PipeReadError     	   // Set status to PipeReadError
		  return status                 // Yes, signal error.
	  }                                // Done checking for error reading from pipe.
	  // ----------------------------- //
	  // Now we write the data to stdout (6).
	  // ----------------------------- //
	  n,err:=os.Stdout.Write(buf[:numRead]) // Write to stdout
	  if err!=nil{                      // Did we error writing to stdout?
	    log.Err("Error writing to stdout: %v",err) // Yes, return nil object and error.
		status=PipeWriteEndClosed         // Set status to PipeWriteEndClosed
		return status                     // Yes, signal error.
	  }                                 // Done checking for error writing to stdout.
	  if n!=numRead{                    // Did we write all the bytes?
	    log.Err("We read %d bytes but wrote %d bytes",numRead,n) // Yes, return nil object and error.
		status=PipeWriteError             // Set status to PipeWriteError
    return status                     // Reurn status.
	  }                                 // Done checking for bytes written.
	  _,_=os.Stdout.Write([]byte("\n")) // Write a newline to stdout (7)
      log.Inf("Wrote %d bytes to stdout",n) // Log the number of bytes written
      if p.Close()!=nil{                // Did we error closing the pipe?
	    log.Err("Error closing pipe: %v",err) // Yes, return nil object and error.
		status=PipeError                // Set status to PipeError
		return status                   // Yes, signal error.
	  }                                 // Done checking for error closing pipe.
	  if status==Success||status==GotEOF{// No errors?
		break                           // Break out of the loop
	  }                                 // Done checking for errors.
	}                                   // Done reading from the pipe.
    default:                            // We are in the parent process
	    log.Inf("Parent process created.")  // Parent process created
	// -------------------------------- //
	// We are the parent so we will be writing to the pipe. (8)
	// -------------------------------- //
      we,err:=p.GetWriteEnd()             // Get the write end of the pipe
	     if err!=nil{                        // Did we error getting the write end of the pipe?
	        log.Err("Error getting write end of pipe: %v",err) // Yes, log it.
	       return status                     // and, signal error.
	     }                                   // Done checking for error getting read end of pipe.
       p.CloseRead()                       // Close the read end of the pipe
	  // -------------------------------- //
	  // Now we write data to the pipe (9).
	  // -------------------------------- //
	    n,err:=we.Write([]byte(os.Args[1])) // Write to the pipe
	    if err!=nil{                        // Did we error writing to the pipe?
	      log.Err("Error writing to pipe: %v",err) // Yes, return log it.
	      status=PipeWriteError             // Set status to PipeWriteEndClosed
	      return status                     // and, signal error.
	    }                                   // Done checking for error writing to pipe.
	    if n!=len(os.Args[1]){              // Did we write all the bytes?
	      log.Err("We read %d bytes but wrote %d bytes",len(os.Args[1]),n) // Yes, return log it..
	      status=PipeWriteError             // Set status to PipeWriteError
	      return status                     // Yes, signal error.
	    }                                   // Done checking for bytes written.
	// -------------------------------- //
	// Now we close the write end of the pipe (10) so child sees EOF.
	// -------------------------------- //
	    if p.CloseWrite()!=nil{             // Did we error closing the write end of the pipe?
	      log.Err("Error closing write end of pipe: %v",err) // Yes, log it.
	      status=PipeWriteEndClosed         // Set status to PipeWriteEndClosed
	      return status                     // Yes, signal error.
      }                                   // Done closing write fd
  // -------------------------------- //
	// Now we wait for the child to terminate (11).
	// -------------------------------- //
	  _,err=syscall.Wait4(int(pid),nil,0,nil) // Wait for the child to terminate
	  if err!=nil{                       // Did we error waiting for the child to terminate?
	    log.Err("Error waiting for child: %v",err) // Yes, return nil object and error.
	    status=UnknownError              // Set status to UnknownError
	    return status                    // Yes, signal error.
	  }								                 // Done checking for error waiting for child to terminate.
    log.Inf("Child terminated.")     // Child terminated successfully
	  if status==Success{              // No errors?
	    log.Inf("PipeToChild completed successfully.") // it's a success.
	    break                          // Break out of the loop
	  }                                // Done checking for child process.
  }                                  // Done handling myself and child.
  return status                      // Return the status code                    
}                                    // ------------ pipeToChild ----------- //

func main() {
  if len(os.Args) < 2 || os.Args[1] == "--help" { // User asking for help?
    fmt.Printf("Usage: %s <string>\n",os.Args[0]) // Print usage message
    os.Exit(1)                          // Yes exit program.
  }                                     // Done checking for help.
  log,err:=logger.NewLogger()           // Create a new logger object
  if err!=nil{                          // Error creating logger?
    fmt.Fprintf(os.Stderr,"error creating logger: %v\n",err)
    os.Exit(1)                          // Yes, exit program.
  }                                     // Done creating logger object.
 // ----------------------------------- //
 // No matter how we exit the program we need to close the logger.
 // So we can clean the semaphore.
 // ----------------------------------- //
  ctx,cancel:=context.WithCancel(context.Background()) // Create a context
  defer cancel()						// Defer canceling the context
  utils.SignalHandler(cancel)		    // Set up signal handler
  utils.RegisterShutdownCB(func(){      // Register shutdown callback
    log.Inf("Shutdown callback called.")
    log.Shutdown()                      // Shutdown the logger
  })                                    // Done registering shutdown callback
  buf:=make([]byte,BUF_SIZE)            // Create a buffer for reading from the pipe
  utils.SetLogger(log)				    // Set the logger object
  status:=pipeToChild(buf,log)          // Call the pipeToChild function
  if status!=Success{                   // Did we error in the pipeToChild function?
    log.Err("Error in pipeToChild: %v",StatusToString(status)) // Yes, return nil object and error.
    // fall through so we hit the shutdown callback.
	cancel()                            // Yes, exit program.
  } else{                               // Else no errors.
    log.Inf("PipeToChild completed successfully.") // PipeToChild completed successfully
	cancel()                            // Cancel the context.
  }                                     // Done checking for errors.
  log.Inf("Exiting program.")           // Exiting program
  <-ctx.Done()                          // Wait for the context to be canceled.
  utils.InvokeShutdownCBs()             // Run the shutdown callbacks.
}                                       // ------------ main ----------------- //