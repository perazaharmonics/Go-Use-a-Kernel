/**
* File: main.go
* Description: This program uses the techniques describes to connect filters
*              together using pipes. After bulding the pipe, this program
*              forks two child processes. The first child binds its stdout
*              to the write end of the pipe and execs ls. The second child
*              binds its stdin to the read end of the pipe and execs wc.
* Author:
*  J.EP, J. Enrique Peraza
* Reference: The Linux Programming Interface, Michael Kerrisk
*/
package main
import (
  "os"
  "fmt"
  "syscall"
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
	ExecError                           // Exec error
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

func pipeToBrother(log logger.Log) (int){
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
  // Fork the process (2)
  // ---------------------------------- //
  pid,_,errno:=syscall.RawSyscall(syscall.SYS_FORK,0,0,0) // Fork the process
  if errno!=0{                          // Error forking to new process?
	log.Err("Error forking process: %v",errno) // Yes, log the error
	status:=ForkError                   // Set the status code
	return status                       // Return the status code
  }                                     // Done checking for fork error.
  switch pid{                           // Act according to the process ID.
  case 0:                               // We are the first child process.
    log.Inf("First child process pid=%d",os.Getpid())
	pfp.CloseRead()                     // Close the read end of the pipe
	// -------------------------------- //
	// Duplicate stdout on write end of the pipe; close duplicated
	// file descriptor (3)
	// -------------------------------- //
	wfp,err:=pfp.GetWriteEnd()          // Get the write end of the pipe
	if err!=nil{                        // Error getting the write end of the pipe?
	  log.Err("Error getting write end of pipe: %v",err) // Yes, log the error
	  status:=PipeWriteEndClosed        // Set the status code
	  return status                     // Return the status code
	}                                   // Done checking for write end of pipe error.
    if wfp.Fd()!=os.Stdout.Fd(){        // Is the write end of the pipe not stdout?
	  _,err=pipe.Dup2File(wfp,int(os.Stdout.Fd())) // Yes, duplicate the write end of the pipe on stdout
	  if err!=nil{                      // Error duplicating the write end of the pipe?
		log.Err("Error duplicating write end of pipe: %v",err) // Yes, log the error
		status:=PipeWriteEndClosed      // Set the status code
		return status                   // Return the status code
	  }                                 // Done checking for write end of pipe duplication error.
	  log.Err("Write end of pipe already bound to stdout") // Log the error
	  if pfp.CloseWrite()!=nil{         // Error closing the write end of the pipe?
		log.Err("Error closing write end of pipe: %v",err) // Yes, log the error
		status:=PipeWriteEndClosed      // Set the status code
		return status                   // Return the status code
	  }                                 // Done closing the write end of the pipe.
	}                                   // Done checking for write end of pipe not stdout.
    // -------------------------------- //
	// Now we use execlp to execute the ls command (4) and write to the pipe
	// -------------------------------- //
    args:=[]string{"ls","-l"}           // Arguments for the ls command
	log.Inf("Executing ls command with args: %v",args) // Log the arguments
	err=syscall.Exec("/bin/ls",args,os.Environ()) // Execute the ls command
	if err!=nil{                        // Error executing the ls command?
	  log.Err("Error in child with pid=%ld executing ls command: %v",os.Getpid(),err) // Yes, log the error
	  status=ExecError                  // Set the status code
	  return status                     // Return the status code	
    }                                   // Done checking for ls command execution error.
  default:                              // Parent falls through to create next child
    log.Inf("Parent process pid=%d",os.Getpid()) // Log the parent process ID	
  }                                     // Done acting according to process ID.
  // ---------------------------------- //
  // Fork the process to create a sibling process that will consume what was
  // written to the pipe by the first child process (5)
  // ---------------------------------- //
  pid,_,errno=syscall.RawSyscall(syscall.SYS_FORK,0,0,0) // Fork the process
  if errno!=0{                          // Error forking to new process?
	log.Err("Error forking process: %v",errno) // Yes, log the error
	status:=ForkError                   // Set the status code
	return status                       // Return the status code
  }                                     // Done checking for fork error.
  switch pid{                           // Act according to the process ID.
  case 0:                               // We are the second child process.
    log.Inf("Second child process pid=%d",os.Getpid()) // Log the second child process ID
	// -------------------------------- //
	// Close the write end of the pipe, we are only interested in the read end
	// because we are the consumer.
	// -------------------------------- // 
	if pfp.CloseWrite()!=nil{           // Error closing the write end of the pipe?
      log.Err("Error closing write end of pipe: %v",err) // Yes, log the error
	  status:=PipeWriteEndClosed        // Set the status code
	  return status                     // Return the status code
	}                                   // Done closing the write end of the pipe.
	// -------------------------------- //
	// Duplicate stdin on read end of the pipe; close duplicated 
	// file descriptor (6)
	// -------------------------------- //
	rfp,err:=pfp.GetReadEnd()           // Get the read end of the pipe
	if err!=nil{                        // Error getting the read end of the pipe?
	  log.Err("Error getting read end of pipe: %v",err) // Yes, log the error
	  status:=PipeReadEndClosed         // Set the status code
	  return status                     // Return the status code
	}                                   // Done checking for read end of pipe error.
	if rfp.Fd()!=os.Stdin.Fd(){         // Is the read end of the pipe not stdin?
	  _,err=pipe.Dup2File(rfp,int(os.Stdin.Fd())) // Yes, duplicate the read end of the pipe on stdin
	  if err!=nil{                      // Error duplicating the read end of the pipe?
		log.Err("Error duplicating read end of pipe: %v",err) // Yes, log the error
		status:=PipeReadEndClosed       // Set the status code
		return status                   // Return the status code
	  }                                 // Done checking for read end of pipe duplication error.
	  log.Err("Read end of pipe already bound to stdin") // Log the error
	  if pfp.CloseRead()!=nil{          // Error closing the read end of the pipe?
		log.Err("Error closing read end of pipe: %v",err) // Yes, log the error
		status:=PipeReadEndClosed       // Set the status code
		return status                   // Return the status code
	  }                                 // Done closing the read end of the pipe.       
    }                                   // Done checking for read end of pipe not stdin.
    // -------------------------------- //
	// Now we use execlp to execute the wc command (7) and read from the pipe
	// -------------------------------- //
	args:=[]string{"wc","-l"}           // Arguments for the wc command
	log.Inf("Executing wc command with args: %v",args) // Log the arguments
	err=syscall.Exec("/usr/bin/wc",args,os.Environ()) // Execute the wc command
    if err!=nil{                        // Error executing the wc command?
	  log.Err("Error in child with pid=%ld executing wc command: %v",os.Getpid(),err) // Yes, log the error
	  status=ExecError                  // Set the status code
	  return status                     // Return the status code
	}                                   // Done checking for wc command execution error.
  default:                              // Parent falls through to wait for children
    log.Inf("Parent process pid=%d",os.Getpid()) // Log the parent process ID
  }                                     // Done acting according to process ID.
  // ---------------------------------- //
  // Wait for the children to finish and close pipes first (8)
  // ---------------------------------- //
  if pfp.Close()!=nil{                  // Error closing the read end of the pipe?
	log.Err("Error closing read end of pipe: %v",err) // Yes, log the error
	status:=PipeError                   // Set the status code
	return status                       // Return the status code
  }                                     // Done closing the read end of the pipe.
  log.Inf("Pipe closed successfully")   // Log the pipe closure
  _,err=syscall.Wait4(int(pid),nil,0,nil) // Wait for the child process to finish
  if err!=nil{                          // Error waiting for the child process?
    log.Err("Error waiting for child process: %v",err) // Yes, log the error
	status:=UnknownError                // Set the status code
	return status                       // Return the status code
  }                                     // Done checking for wait error.
  log.Inf("Child process finished successfully") // Log the child process finish
  return status                         // Return the status code
}                                       // ------------ pipeToBrother --------- //

func main(){
  log,err:=logger.NewLogger()           // Create a new logger
  if err!=nil{                          // Error creating logger?
    fmt.Fprintf(os.Stderr,"error creating logger: %v\n",err)
	os.Exit(1)                          // Yes, exit program.
  }                                     // Done creating logger object.
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
  utils.SetLogger(log)				    // Set the logger object  
  log.Inf("Starting pipeToBrother")     // Log the start of the program
  status:=pipeToBrother(log)        // Call the pipeToBrother function
  if status!=Success{   // Report the error
    log.Err("Pipe to child process state returned error: %s",StatusToString(status))
  } else{                               // The good ending.
    log.Inf("Pipe to child process state returned: %s",StatusToString(status))
  }                                     // Done logging.
  cancel()                              // Send context cancellation signal.
  log.Inf("Program exited.")            // Log goodbye.
  utils.InvokeShutdownCBs()             // Nice cleanup.  
}