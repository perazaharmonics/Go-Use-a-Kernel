/****************************************************************************
* Filename:
*	 checklogfile.go
*
* Description: 
*  A program to determine which processes were using the Go log file and send
*	them a SIGHUP signal to tell them to re-open the log file.
*
* Author: J.EP, J. Enrique Peraza
*******************************************************************************/

package checklogfile

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"strconv"
	"syscall"
)

// Helper functions to build sentences. Should probably make an English package.
// Much like Matt's str.h/cpp in the C++ version.

func pls(count int) string {
  if count == 1 {
	  return ""
	}
	return "s"
}

func was(count int) string {
  if count == 1 {
	  return "was"
	}
	return "were"
}


func ples(count int) string {
  if count == 1 {
	  return ""
	}
	return "es"
}

// CheckLogFile finds processes using the log or error files, sends them a SIGHUP
// and reports counts for "tail", "less" and "more", and other processes using these
// files.
func CheckLogFile() {                   // -----------CheckLogFile---------- //
	
	// ---------------------------------- //
	// Get the log file pathname.         
	// ---------------------------------- //
	ngo:=os.Getenv("NGO")                 // Get the NetGo environment variable.
	if ngo == "" {                        // No NetGo environment variable?
	  fmt.Printf("NetGo environment variable not set.\n")
		os.Exit(1)                          // No use for this program then, exit.
	}                                     // Done with "no $NGO" error.
	logp:=ngo+"/logs/log.txt"             // Set the log file pathname.
	//logp2:=logp+"(deleted)"               // Set the deleted log file pathname.
  // ---------------------------------- //
	// Check if the log file exists.
	// ---------------------------------- //
	if _,err:=os.Stat(logp); os.IsNotExist(err) {// Does the log file exist?
	  fmt.Printf("Log file %s does not exist.\n", logp)
		os.Exit(1)                          // No use for this program then, exit.
	}                                     // Done checking for log file existence.
	
  
	// Print the command being executed.
	fmt.Printf("Running command: lsof %s.\n", logp)
	// ---------------------------------- //
	// Use the lsof program to see what processes have "log.txt" open.
	// ---------------------------------- //
	cmd := exec.Command("lsof",logp)      // Create the lsof command.
	stdout, err := cmd.StdoutPipe()       // Get the stdout pipe.
	if err != nil {                       // Error getting stdout pipe?
			fmt.Printf("Error opening lsof pipe: %v\n", err)
			os.Exit(1)                        // No use for this program then, exit.
	}                                     // Done checking for error getting stdout pipe.
	if err := cmd.Start(); err != nil {   // Error starting lsof?                  // Else, some other error starting lsof.
		fmt.Printf("Error starting lsof: %v\n", err)
		os.Exit(1)                          // No use for this program then, exit.
	}                                     // Done checking for error starting lsof.
  var n, tail, less, more int           // Our counters.
  scanner:=bufio.NewScanner(stdout)     // Our scanner for the lsof output.
	for scanner.Scan() {                  // While we can scan the lsof output.
	  line:=scanner.Bytes()               // Get the incoming line.
		//fmt.Println("DEBUG: lsof output line:", string(line))
		// Check if the line contains the log file or error file.
		if bytes.Contains(line,[]byte(logp)) { // Does the line contain the log file?
		// -------------------------------- //
		// If so, we need to extract the process name and process ID.
		// First we need to find the first space in the line.
		// This is the space between the process name and the process ID.
		// So we can extract the tokens we care about.
		// -------------------------------- //
			sp:=bytes.IndexByte(line, ' ')    // Extract the process name (first token)
      if sp == -1 {                     // Found a space? 
			  continue                        // No, skip this line.
			}                                 // Done checking for space.
			// Otherwise we have a process name.
			proc:=bytes.TrimSpace(line[:sp])  // Extract everything up to 1st space.
			p:=sp+1                           // Skip the space.
			for p<len(line) && (line[p] == ' ' || line[p] == '\t'){// While line[p] is a space.
			  p++                             // Skip spaces.
			}                                 // Done skipping spaces.
			// The next token should be the process ID.
			pidstr:=line[p:]                  // Extract everything starting at p.
			endpid:=bytes.IndexByte(pidstr, ' ')// Find the end of the process ID token.
			if endpid != -1 {                 // Found the end of the process ID?
			  pidstr=pidstr[:endpid]					// Yes, extract everything up to the end.
			}                                 // Done checking for end of process ID.
			pid,err:=strconv.Atoi(string(pidstr))// Turn pidstr into an integer.
			if err != nil {                   // Error converting pidstr to integer?
			  fmt.Printf("Could not conver pidstr to int: %v\n", err)
			}                                 // Done with str to int conversion err.
      // So now we have a process ID that we can send a SIGHUP to
			// Except for "tail", "less" and "more".
			if bytes.Equal(proc, []byte("tail")) {// Found a "tail" process?
			  tail++                          // Yes, just count it's occurrence.
			} else if bytes.Equal(proc, []byte("less")) {// Found a "less" process?
			  less++                          // Yes, just count it's occurrence.
			} else if bytes.Equal(proc, []byte("more")) { // Found a "more" process?
			  more++                          // Yes, just count it's occurrence.
			} else { // Else, it's some other process, for which we can send and trap a SIGHUP.
			  fmt.Printf("Sending SIGHUP to process %d (%s)\n", pid, proc)
				if err:=syscall.Kill(pid, syscall.SIGHUP); err != nil {
				  fmt.Printf("Error sending SIGHUP to process %d: %v\n", pid, err)
				}                               // Done sending SIGHUP and checking for error.
				n++                             // Increment the count for other processes.
			}                                 // Done checking for "tail", "less" and "more".
		}                                   // Done checking for log file usage.  		
	}                                     // Done scanning the lsof output.
	if err :=scanner.Err(); err != nil {  // Error scanning lsof output?
	  fmt.Printf("Error reading lsof output: %v\n", err)
		os.Exit(1)                          // No use for this program then, exit.
	}                                     // Done checking for error scanning lsof output.
	if err := cmd.Wait(); err != nil {    // Error waiting for our child process? Wow )^;
		// If the exit code is 1, then no processes are using the log files.
		// So that's actually not a bad thing.
		//fmt.Printf("Error waiting for lsof: %v\n", err) // Not cool...
		//os.Exit(1)                        // No use for this program then, exit.		
		if exitErr,ok:=err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			fmt.Printf("No processes using the log files.\n")
			os.Exit(0)                        // Return normally.
		} else {                            // Else, error waiting for child process.
		  fmt.Printf("Error waiting for lsof: %v\n", err) // Not cool...
		  os.Exit(1)                        // No use for this program then, exit.
		}
	}                                     // Done checking for error waiting for child process.
	// ---------------------------------- //
	// Now we have the counts for "tail", "less" and "more" and other processes.
	// So now we have to construct the summary message, which is a bit tricky.
	// without a ternary operator.
	// ---------------------------------- //
	// We have to account for the number of "tail", "less" and "more" processes.
	// We also have to account for any extra processes.
	// First we have to build a parenthetical summary relative
	// to the number of "tail", "less" and "more" processes.
	// Then we have to account for any extra processes.
	// ---------------------------------- //
	var tailstr strings.Builder           // Our string builder for the tail summary.
	if tail+less+more > 0 {               // Any "tail", "less" or "more" processes?
	  if n > 0 {                          // ... and extra processes to account for?
		  tailstr.WriteString("(and")       // Yes, so add "and" to the string.
		} else {                            // Else, no extra processes to account for.
		  tailstr.WriteString("(except for")// So add "except for" to the string.
		}                                   // Done with additional processes.
		if tail > 0 {                       // Did we count any tail processes?
		  tailstr.WriteString(fmt.Sprintf(" %d instance%s of \"tail\"",tail,pls(tail)))
		}
		if less > 0 {                       // Did we count any "less" processes?
		  if tail > 0 {                     // Did we also count any "tail" processes?
			  if more == 0 {                  // But no "more" processes?
				  tailstr.WriteString(" and")   // Yes, so add an "and" to the string.
				} else {                        // Else, we have "more" processes.
				  tailstr.WriteString(",")      // So add a comma to the string.
				}                               // Done checking for "more" processes.
			}                                 // Done checking for "tail" processes.
			tailstr.WriteString(fmt.Sprintf(" %d instance%s of \"less\"",less,pls(less)))
		}                                   // Done checking for "less" processes.
		if more > 0 {                       // Did we count any "more" processes?
		  // Now we have to determine if we need to add an Oxford comma.
			if tail == 0 && less == 0 {       // No "tail" or "less" processes?
			  tailstr.WriteString("")         // NOOP
			} else if less == 0 {             // No "less" processes?
			  tailstr.WriteString(" and")     // Add an "and" to the string.
			} else {                          // Else we have a "less" process and maybe a "tail" process.
			  tailstr.WriteString(", and")    // Add the Oxford comma
			}                                 // Done determining punctuation.
			tailstr.WriteString(fmt.Sprintf(" %d instance%s of \"more\"",more,pls(more)))
		}                                   // Done checking for "more" processes.
		tailstr.WriteString(") ")            // Close the parenthetical summary and pray.
	}                                     // Done with the Philosophy of Language and Logic.
	// ---------------------------------- //
	// So now with all the parts a summary message can be constructed.
	// ---------------------------------- //
	if n == 0 {                           // Any NOT tail, less, and more procs using logs?
	  fmt.Printf("No process%s %s%s using the log files.\n",
		  ples(tail+less+more),tailstr.String(), was(tail+less+more))
	} else {                              // Else, other processes we using the log files.
	  fmt.Printf("%d process%s %s%s using the log files.\n",
		  n,ples(n),tailstr.String(),was(n+tail+less+more))
	}                                     // Done checking for other processes using log files.
}                                       // -----------CheckLogFile---------- //
