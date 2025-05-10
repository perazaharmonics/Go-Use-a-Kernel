/****************************************************************
* filename:
*  logger.go
* Description:
*  A wrapper function for the Go log package, such that it records
*  the timestamp, the file name, function name and if it's an error
*  the error code, error symbol and the error message.
* Author:
*  JEP  J.Enrique Peraza
***************************************************************/

package logger

import (
    "fmt"
    "os"
    "sync"
    "strings"
    "path/filepath"
    "runtime"
    "time"

    semaphore "github.com/perazaharmonics/project_name/internal/semaphore"

)
// ------------------------------------ //
// Helper function to get the current function name
// ------------------------------------ //
func getFuncName() string {             // -----------getFuncName-------- //
	// Get the program counter, file name, line number and ok value.
	pc, _, _, _ := runtime.Caller(3)      // We just want the current func name.
    // -------------------------------- //
    // Delete everything but the function name of the caller, and name
    // -------------------------------- //
    fName:= runtime.FuncForPC(pc).Name()// Get the function name
    // -------------------------------- //
    // Split the function name by the last dot and return the last part
    // -------------------------------- //
    lastDot := len(fName)-1             // Get the length of the function name
    for i := len(fName)-1;i >= 0;i--{   // Iterate over the function name
      if fName[i]=='.'{                 // If we find a dot
        lastDot=i                       // Set the last dot to the current index
        break                           // Break the loop
      }                                 // Otherwise, continue.
    }                                   // Done iterating over the function name
    // -------------------------------- //
    // Return the function name after the last dot
    // -------------------------------- //
    fName=fName[lastDot+1:]             // Get the function name after the last dot
    // -------------------------------- //
    // Check if the function name is empty
    // -------------------------------- //
    if fName==""{                       // If the function name is empty
      return "unknown"                  // Return unknown
    }                                   // Otherwise, continue.
    // -------------------------------- //
    // Check if the function name is a method
    // -------------------------------- //
    if fName[len(fName)-1] == '}'{      // If the function name is a method
      fName = fName[:len(fName)-1]      // Remove the last character
    }                                   // Otherwise, continue.
	return fName                          // return function name
}                                       // -----------getFuncName-------- //


// Helper function to get the current file name
func getAppname() string {             // -----------getAppname-------- //
	// Get the program counter, file name, line number and ok value.
	_, file, _, _ := runtime.Caller(3)  // We just want the current filename. 
	return filepath.Base(file)          // Return the file name. 
}                                       // -----------getAppname-------- //


// Helper function to get the current line number
func getLineNumber() int {             // -----------getLineNumber-------- //
    // Get the program counter, file name, line number and ok value.
    _, _, line, _ := runtime.Caller(3)   // We jsut want the line number.   
    return line                          // Return the line number.
}
// --------------------------------------------------------------------------
// The actual log object
// --------------------------------------------------------------------------
const (
    logFilename = "log.txt"             // Log file name
    errFilename = "error.txt"           // Error file name
    maxLogSize=64*1024*1024             // Max log file size is 64 MiB
)
var (
logdirname = ""
logpathname string = ""
errpathname string = "/home/ljt/Projects/NetGo/logs/error.txt" 
fpl *os.File = nil                      // Pointer to the log file.
fpe *os.File = nil                      // Pointer to the error file.
sem *semaphore.Semaphore=nil            // Pointer to the semaphore.
once sync.Once = sync.Once{}     // Used to ensure we call destructor only once
)


// LogLevel defines the log level
type LogLevel int
const (
	// LogLevelDebug is the debug log level 
	//(iota is used to create a sequence of constants)
	Debug LogLevel = iota               // Iota 0 
	Info                                // Info level 1
	Warning                             // Warning level 2
	Error                               // Error level 3
	Fatal                               // Fatal level 4
)

// Logger is a wrapper for the Go log package
type Logger struct {
    mu     sync.Mutex                   // Mutex to protect the log file
    Level  LogLevel                     // Log level
    Symbol string                       // Annunciatior to indicate level.
    init   bool                         // Flag to indicate if logger was init.
}
// ------------------------------------- //
// NewLogger creates a new logger instance and initializes it. Return the object
// and nil error is successfull, else return nil and the error.
// -------------------------------------- //
func NewLogger() (*Logger,error){       // ------------ NewLogger ----------- //
   l:=&Logger{                          // Our new logger instance.
    Level: 0,                           // Set the log level
    Symbol: "",                         // Set the symbol to empty
    mu: sync.Mutex{},                   // Initialize the mutex
    init: false,                        // Set the init flag to false.
  }                                     // Return the logger instance
  if err:=l.Initialize();err!=nil{      // Error initializing the logger?
    return nil,err                      // Yes, return nil object and the error.
  }                                     // No error, continue.
  return l,nil                          // Return the logger instance and no error.
}                                       // ------------ NewLogger ----------- // 

// ----------------------------------------------------------------------------
// Initializer is meant to be called by NewProxyServer() to initialize the logger
// and the semaphore, as well as other itialization routines.
// -----------------------------------------------------------------------------
func (l *Logger) Initialize() error{    // ----------- Initialize ----------- //
  exe,err:=os.Executable()              // Ask for Go running binary's path.
  if err!=nil{                          // Error getting the executable?
    return fmt.Errorf("InitLog: cannot determine executable: %w.",err)
  }                                     // Done with error getting executable.
  if real,err:=filepath.EvalSymlinks(exe);err==nil{// Is the executable a symlink?
    exe=real                            // Yes, resolve it and set it.              
  }                                     // Done checking and dereferencing symlinks.
  appname:=getAppname()                 // The app that is calling the logger.
  var semerr error                      // Semaphore error.
  sem,semerr=semaphore.NewSemaphore(appname,"log","pharm",0x7003)// Make a semaphore.
  if semerr!=nil{                       // Error creating semaphore?
    return fmt.Errorf("InitLog: cannot create semaphore: %w.",err)
  }                                     // Done checking for err with semaphore.
  proxy:=os.Getenv("PROXY")             // Get the $PROXY symbol's value.
  if proxy==""{                         // Is the proxy symbol set?
    return fmt.Errorf("InitLog: The $PROXY environment variable is nor defined.")
  }                                     // Done checking for the proxy symbol.
  logdirname=fmt.Sprintf("%s/logs",proxy)// Set the directory name.
  logpathname=fmt.Sprintf("%s/%s",logdirname,logFilename)// Set the log file name.
  errpathname=fmt.Sprintf("%s/%s",logdirname,errFilename)// Set the error file name.
  l.init=true                           // Set the init flag to true.
  return nil                            // Return nil error if successfull.
}                                       // ----------- Initialize ----------- //

func openLogFile() error{                     // Open the log file.
  var err error                         // Error opening the log file?
  fpl,err= os.OpenFile(logpathname, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
  if err!=nil{                          // Could we open the log file?
    return fmt.Errorf("openlogfile: failed to open file: %w", err)
  }                                     // Done checking for error opening log.
  // ---------------------------------- //
  // We should have already built the logpathname, but maybe we forgot to call
  // Initialize() so we try and check again as a defensive measure.
  // ---------------------------------- //
  if fpl == nil {                       // Is the log file open?
    logpathname=fmt.Sprintf("%s/%s",logdirname,logFilename)
    fpl,err=os.OpenFile(logpathname, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
    if err != nil {                     // Ok now this must really be an error.                 
      return fmt.Errorf("openlogfile: failed to open file: %v\n", err)
    }                                   // Done checking for error opening log.
  }                                     // Done checking if the log file is open.
  info,err:=os.Stat(logpathname)        // Get the file info.
  if err!=nil{                          // Error getting file info?
    openErrorfile();                    // Open the error file.
    fmt.Fprintf(fpe, "Can't stat log file. \"%s\": %v .\n",
      logpathname, err);                // Log that in the error file.
    fmt.Fprintf(fpe, "Fatal error in Logger(), exiting.\n")
    return fmt.Errorf("openlogfile: Can't stat log file: %w.",err)                              // Exit the function.
  } else{                               // Else we could stat the file.
    siz:=info.Size()                    // Get the file size.
    if siz>=maxLogSize{                 // Have exceeded the max log size?
      alreadyDone:=false                // True if proc already renamed.
      fmt.Fprintf(fpl,"%s%d %s%s *** Log file has exceeded maximum size limit of %d bytes. ***\n",
        time.Now().Format(time.RFC3339), siz, getAppname(), getFuncName(), maxLogSize)
      if !alreadyDone{                  // If we haven't already renamed the file.
        var newlogpathname string       // New log file name.
        if logpathname[0]== 0{          // Do we have a pathname yet?
          logpathname=fmt.Sprintf("%s/%s", logdirname, logFilename)
        }                               // Done checking for pathname.
        dt:=time.Now()                  // Get the current date and time.
        newlogpathname+=fmt.Sprintf("%s/log_%s.txt",logdirname, dt.Format(time.RFC3339Nano))
        err:=os.Rename(logpathname, newlogpathname)
        if err!=nil{                    // Error renaming the log file?
          return fmt.Errorf("openlogfile: Failed to rename log file: %w", err)
        }                               // Done checking for error renaming log.
      }                                 // Done checking if we have already renamed the file.
      fpl,err=os.OpenFile(logpathname, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
      if err!=nil{                      // Error opening the new log file?
        return fmt.Errorf("openlogfile: Failed to open new log file: %v\n", err)
      }                                 // Done checking for error opening new log file.
    }                                   // Done checking if we have exceeded the max log size.
  }                                     // Done checking if we could stat the log file.
  return nil                            // Return nil error if successfull.
}                                       // ---------- openLogFile ----------- //

func openErrorfile() error{             // Open the error file.
  var err error                         // Error opening the error file?
  // ---------------------------------- //
  // Open the error file                //
  // ---------------------------------- //
  fpe,err=os.OpenFile(errpathname, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
  if err!=nil{                          // Error opening the error file?
    return fmt.Errorf("openerrorfile: Failed to open error file: %v\n", err)
  }                                     // Done checking for error opening error file.
  if fpe==nil{                          // Is the error file open?
    // Comment out this block when you've created an App class and Server class.
    // For now we are working without inheritance, and we are a server so
    // we'll write to a log file. In Matt's code, it looks something like this:
    // if (appType != APP_TYPE_SERVER)
    //  fmt.Printf("Failed to get file info: %v\n", err)
    //  fmt.Printf("Fatal error in Loggger.openLogFile(), exiting...\n")
    // } 
    return fmt.Errorf("openlogfile: Error %w opening the error log file %s",
      err, errpathname)                 // Log that in the error file.  
  }                                     // Done checking if the error file is open.
  return nil                            // Return nil error if successfull.
}                                       // --------- openErrorfile ---------- //

// ------------------------------------ //
// Shutdown flushes & closes all log handles and releases the semaphore exactly
// once using the sync.Once package. This is to ensure that we don't have multiple
// goroutines trying to close the log files at the same time, which would cause
// a panic if they are already closed and try to dereference a nil pointer.
// ------------------------------------ //
func (l *Logger) Shutdown() error{      // ----------- Shutdown ------------- //
  var shuterr error                     // Where to store the shutdown error.
  once.Do(func(){                       // Do the following exactly once.                     
    if fpl!=nil{                        // Is the log file open?
      l.ExitLog("Because we are shutting down the logger.")// Yes, close it.
    }                                   // Done checking if the log file is open.
    if fpe!=nil{                        // Is the error file open?
      l.ExitLog("")                     // Yes, close it.
    }                                   // Done checking if the error file is open.
    if sem!=nil{                        // Is the semaphore down?
      if err:=sem.Close();err!=nil{     // Could we close it?
        shuterr=fmt.Errorf("shutdown semaphore: %w",err)// No, log that error.
      }                                 // Done trying to lift the semaphore.
      sem=nil                           // Remember we closed the semaphore.
    }                                   // Done checking if we had a semaphore.
  })                                    // Done doing the shutdown.
  return shuterr                        // Return the shutdown error.
}                                       // ----------- Shutdown ------------- //


// =========================================================
// As to not have to call the ExitLog() and a signalHandler
// per every module, creating a parent module like 
// Application.h/cpp (see Matt's code) and Server.h/cpp
// would be a good idea. This would allow us to create 
// many servers and apps that all inherit common functionality
// from the parent class, and minimize code repetition.
// =========================================================
func (l *Logger) ExitLog(format string, args ...interface{}) {
    // -------------------------------- //
    // Check if the log file is open, and if we have a format string
    // the tells us why we are closing the log file.
    // -------------------------------- //
  openErrorfile()                       // Open the error file.
  if fpl != nil {                       // Is the log file open?
    if format != "" {                   // .. and we have a formatted reason why?
      msg:=fmt.Sprintf(format, args...) // Format the message
      fmt.Fprintf(fpl, "%s %s Closing all log files %s.\n",
        time.Now().Format(time.RFC3339Nano),getAppname(),msg)
    } else {                            // Else we were not told why.
        fmt.Fprintf(fpl, "%s %s Closing all log files.\n",
            time.Now().Format(time.RFC3339Nano),getAppname())
    }                                   // Done with no reason why.
    fpl.Close(); fpl=nil                // Close the log file.
  }                                     // Done closing the log file.
    // -------------------------------- //
    // Check if the error log file is open, and if we have a format string
    // the tells us why we are closing the log file.
    // -------------------------------- //  
  if fpe != nil {                       // Is the error file open?
    if format != "" {                   // .. and we have a formatted reason why?
      msg:=fmt.Sprintf(format, args...) // Format the message
      fmt.Fprintf(fpe, "%s %s Closing all log files %s.\n",
        time.Now().Format(time.RFC3339Nano),getAppname(),msg)
    } else {                            // Else we were not told why.
        fmt.Fprintf(fpe, "%s %s Closing all log files.\n",
            time.Now().Format(time.RFC3339Nano),getAppname())
    }                                   // Done with no reason why.
    fpe.Close(); fpe=nil                // Close the error file.
  }                                     // Done closing the error file.
}                                       // ------------- ExitLog ------------ //
// ------------------------------------ //
// Function to clear the log file, before writing to it
// ------------------------------------ //
func (l *Logger) clearLogFile (file string) {
  os.OpenFile(file, os.O_RDWR|os.O_TRUNC, 0644) // Open the file in truncate mode                                  
}                                       // ---------- clearLogFile ---------- //
// ------------------------------------ //
// Function to write to the log file
// ------------------------------------ //
func (l *Logger) writeToFile(file,msg string) error{
  // Open the file in append mode, create it if it doesn't exist
  if file==logpathname{                 // Is the file the log file?
    openLogFile()                       // Yes, open the log file.
    _, err:=fpl.WriteString(msg)        // Write the log message to the file
    if err!=nil{                        // Error writing to the file?
      return fmt.Errorf("writetofile(%q): %w",file,err)
    }                                   // Otherwise, continue.
    fpl.Sync()                          // Sync the file to ensure all data is written      
  } else {                              // Open the error file in append mode, create it if it doesn't exist
      openErrorfile()                   // Open the error file.
      _, err := fpe.WriteString(msg)    // Write the log message to the file
      if err!= nil{                     // Error writing to the file?
        return fmt.Errorf("writetofile(%q): %w",file,err)
      }                                 // Else, continue
      fpe.Sync()                        // Sync the file to ensure all data is written
  }                                     // Done checking which file to write to.
  return nil                            // Return nil error if successfull.
}                                       // ---------writeToFile-------- //

// logMessage is the internal log function that facilitates writing logs
// to the specified text file.
func (l *Logger) logMessage(level LogLevel, msg string) {
  if level < l.Level {                  // Log level less than current level?
	  return                              // If so, return without logging.
	}                                     // Otherwise, continue.
  switch level {                        // Set the symbol based on the log level
    case Debug:                         // Debug level?
      l.Symbol = "[DEBUG] "             // Set symbol to [DEBUG]
    case Info:                          // Info level?
      l.Symbol = ""                     // Set symbol to none
    case Warning:                       // Warning level?
      l.Symbol = "* "                   // Set symbol to *
    case Error:                         // Error level?
      l.Symbol = "! "                   // Set symbol to !
    case Fatal:                         // Fatal level?
      l.Symbol = "@ "                   // Set symbol to !!
  }                                     // Done setting the symbol
  // ---------------------------------- //
  // If the message in the buffer is a multiline message we will purge that
  // buffer and set a recursive entrypoint so that it enters the log
  // with a clean and nice buffer. 
  // ---------------------------------- //
  if strings.Contains(msg,"\n"){        // Does the buffer container a newline?
    for _,line:=range strings.Split(msg,"\n"){ // Yes split them by line & purge.
      if line!=""{                      // Is the purged message not empty?
        l.logMessage(level,line)        // Log that message without the newline.
      } else {                          // Otherwise...
        continue                        // Skip the empty line.
      }                                 // Done checking the line.
    }                                   // Done splitting the message.
    return                              // Return if we had to purge a message.
  }                                     // Otherwise no newline so just fall through.
  // ---------------------------------- //
  // Lock the mutex so that you have a goroutine-local lock
  // and unlock it when done.
  // ---------------------------------- //
  l.mu.Lock()                           // Lock the mutex to protect the log file
  defer l.mu.Unlock()                   // Unlock the mutex after writing
  // ---------------------------------- //
  // Lock the semaphore to ensure only one process can write to the file at a time
  // and unlock it when done.
  // ---------------------------------- //
  sem.Lock("Because we are writing to the log file.")
  defer sem.Unlock("Because we are done writing to the log file.")
  // -------------------------------- //
  // Get the file size to check if it exceeds 500KiB,
  // if so, clear the log file.
  // -------------------------------- //
  flogInfo, err := os.Stat(logpathname)// Get the file info
  if err != nil {                     // Error getting file info?
    fmt.Printf("Failed to get file info: %v\n", err)// Error getting file info
    return                            // Return if error
  }                                   // Otherwise, continue.
  ferrInfo, ers := os.Stat(errpathname)// Get the error file info
  if ers != nil {                     // Error getting file info?
    fmt.Printf("Failed to get file info: %v\n", ers)// Error getting file info
    return                            // Return if error
  }                                   // Otherwise, continue.
  flogSiz:=flogInfo.Size()            // Get the file size
  if flogSiz > maxLogSize {           // If file size exceeds 30KB
    l.clearLogFile(logpathname)       // Clear the log file
  }                                   // Otherwise, continue.
  ferrSiz:=ferrInfo.Size()            // Get the error file size
  if ferrSiz > maxLogSize {           // If file size exceeds 30KB
    l.clearLogFile(errpathname)       // Clear the error file
  }                                   // Otherwise, continue.
  // ---------------------------------- //
	// Write the log message to the file
	// ---------------------------------- //
  maxcol:=168                           // Maximum column size of the log message.
  timestamp:=time.Now().Format(time.RFC3339) // Get the current timestamp
  filename:=getAppname()               // Get the file name
  funcname:=getFuncName()               // Get the function name
  hdr:=fmt.Sprintf("%s: %s: %s: %s", timestamp, filename, funcname, l.Symbol) // Create the header
  hRunes:=[]rune(hdr)                   // Convert header to slice of runes.
  // ---------------------------------- //
  // Calculate the space for one separator and the body start.
  // ---------------------------------- //
  bWidth:=maxcol-len(hRunes)-1          // Calculate the message body's width
  indent:=strings.Repeat(" ",len(hRunes)+1)// Create the indent
  // ---------------------------------- //
  // Chop the message into chunks of bodyWidth characters (runes)
  // then write them to the logfile.
  // ---------------------------------- //
  bRunes:=[]rune(msg)                   // Convert message to slice of runes.
  first:=true                           // Flag for first line of message.
  line:=""                              // Line to chup up rune slice.
  for len(bRunes)>0{                    // While there are things to write!
    if len(bRunes)>bWidth{              // Is the msg larger than bodyWidth?
      line=string(bRunes[:bWidth])      // Yes get the first bodyWidth runes.
      bRunes=bRunes[bWidth:]            // Remember we removed the first bodyWidth runes.
    } else {                            // Else the msg is smaller than bodyWidth.
      line=string(bRunes)               // Get the runes without chopping them.
      bRunes=nil                        // Remember we removed the remaining runes.
    }                                   // Done checking the length of the runes.
    if first{                           // Is this the first line of the msg?
      out:=hdr+" "+line+"\n"            // The prefix with the header.
      l.writeToFile(logpathname,out)    // Write the log message to the file.
      if level >= Error{                // If the log level is Error or Fatal.
        l.writeToFile(errpathname,out)  // Write the log message to the error file.
      }                                 // Done checking which file(s) to write to.
      first=false                       // Remember we wrote the first line.
    } else {                            // Else this is not the first line.
    // -------------------------------- //
    // So now we have a line that must have a prefix and a body and is greater
    // than 168 runes (characters) long so it might have to be indented.
    // -------------------------------- //
      out:=indent+line+"\n"             // The prefix with the body.
      l.writeToFile(logpathname,out)    // Write the log message to the file.
      if level >= Error{                // If the log level is Error or Fatal.
        l.writeToFile(errpathname,out)  // Write the log message to the error file.
      }                                 // Done writing to the file(s).
    }                                   // Done with long line. 
  }                                     // Done with while we have to write.
}                                       // ---------logMessage-------- //

// Deb logs a debug message
func (l *Logger) Deb(format string, args ...interface{}) bool {
    msg := fmt.Sprintf(format, args...)
    l.logMessage(Debug, msg)
    return true
}

// Inf logs an info message
func (l *Logger) Inf(format string, args ...interface{}) bool {
    msg := fmt.Sprintf(format, args...)
    l.logMessage(Info, msg)
    return true
}

// War logs a warning message
func (l *Logger) War(format string, args ...interface{}) bool {
    msg := fmt.Sprintf(format, args...)
    l.logMessage(Warning, msg)
    return true
}

// Err logs an error message
func (l *Logger) Err(format string, args ...interface{}) bool {
    msg := fmt.Sprintf(format, args...)
    l.logMessage(Error, msg)
    return false
}

// Fat logs a fatal message
func (l *Logger) Fat(format string, args ...interface{}) bool {
    msg := fmt.Sprintf(format, args...)
    l.logMessage(Fatal, msg)
    return false
}
