/*=============================================================================*
* Filename: 
*   signals.go
* 
* Description: 
*   This file contains the Signal Handling traps for the proxy
*   server, as well as support for registering custom shutdown callbacks or exit
*   handlers. It captures the SIGINT/SIGTERM signal for graceful shutdown,
*   and uses the SIGHUP for log rotation. An example of a custom shutdown
*   that can be registered is the HTTP server shutdown.
* 
* Author:
*   J.EP, J. Enrique Peraza
==============================================================================*/
package utils

import (
	"context"   // For context handling
	"os"        // For file operations, I/O, system calls
	"os/signal" // For signal handling
	"sync"      // For mutexes and locks
	"syscall"   // For syscall handling

	logger "github.com/perazaharmonics/gosys/internal/logger" // Our custom log package.
)

var (
	log         logger.Log
	shutdownCBs []func()   // Slice of shutdown callbacks
	mtx         sync.Mutex // Protect shutdownCBs slice.
)

// const debug = true                   // Enables debug logging.
// ------------------------------------ //
// SetLogger pernits our main package to hand over the log object to the
// signal.go package.
// ------------------------------------ //
func SetLogger(l logger.Log) { // ----------- SetLogger ------------ //
	mtx.Lock()         // Lock the mtx to protect the log object.
	defer mtx.Unlock() // Unlock the mtx when done.
	log = l            // Set the log object.
} // ----------- SetLogger ------------ //
// ------------------------------------ //
// GetLogger return the log object used in this package.
// ------------------------------------ //
func GetLogger() logger.Log { // ----------- GetLogger ------------ //
	mtx.Lock()         // Lock the mtx to protect the log object.
	defer mtx.Unlock() // Unlock the mtx when done.
	if log == nil {    // Have we initialized the log object?
		l, _ := logger.NewLogger() // No, create a new logger object.
		log = l                    // Set a default log object.
	} // Done creating the log object.
	return log // Return the log object.
} // ----------- GetLogger ------------ //
// ------------------------------------ //
// RegisterShutdownCB provides a way to register a exit handler or shutdown
// callback function for external packages (e.g. httpserverm proxyd)
// that are run when a SIGINT/SIGTERM signal is received.
// ------------------------------------ //
func RegisterShutdownCB(cb func()) { // ------ RegisterShutdownCB -------- //
	mtx.Lock()                            // Lock the mtx to protect the slice.
	defer mtx.Unlock()                    // Unlock the mtx when done.
	shutdownCBs = append(shutdownCBs, cb) // Append the callback to the slice.
} // ------ RegisterShutdownCB -------- //
// ------------------------------------ //
// SignalHandler sets up a signal listener that handles SIGHUP for log rotation
// SIGINT/SIGTERM for graceful shutdown, and SIGQUIT for immediate exit.
// ------------------------------------ //
func SignalHandler(cancel context.CancelFunc) { // ------- SignalHandler ---------- //
	sigCh := make(chan os.Signal, 1) // A channel to receive OS signals.
	// ---------------------------------- //
	// Notify the channel when we receive these signals.
	// ---------------------------------- //
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT,syscall.SIGPIPE)
	// ---------------------------------- //
	// Spawn a gouroutine that listens for signals and handles them on a separate
	// thread.
	// ---------------------------------- //
	go func() { // On a separate thread.
		for { // Until we receive a signal..
			sig := <-sigCh // Wait for a signal on the channel.
			switch sig {   // Yes, what signal is it?
			case syscall.SIGHUP: // It was a SIGHUP signal.
				log.Inf("Closing the log file.") // Log rotation.
				log.ExitLog("Because we received a SIGHUP signal.")
				log.Inf("Re-opened log file.") // Done handling SIGHUP.
			case syscall.SIGINT, syscall.SIGTERM: // It was a SIGINT/SIGTERM signal?
				log.Inf("Received %v: Starting graceful shutdown.", sig)
				cancel()         // Cancel the context.
				runShutdownCBs() // Run the shutdown callbacks.
				os.Exit(0) // Exit the program.
			case syscall.SIGQUIT: // Is it a SIGQUIT signal?
				log.War("Received SIGQUIT: Forcing shutdown.")
			 cancel()          // Cancel the context.
    runShutdownCBs() // Run the shutdown callbacks.
				os.Exit(0)
			case syscall.SIGPIPE: // Is it a SIGPIPE signal?
				log.War("Received SIGPIPE: Ignoring.")
			default: // It was something else.
				log.Err("Received unknown signal: %v", sig)
				cancel() // Cancel the context.
    runShutdownCBs() // Run the shutdown CBs
    os.Exit(1)
			} // Done checking the signal.
		} // Done waiting for signals.
	}() // Done spawning the goroutine.
} // ---------- SignalHandler --------- //
// ------------------------------------ //
// runShutdownCBs runs all of the registered shutdown callback functions in the
// order they were registered.
// -- ---------------------------------- //
func runShutdownCBs() { // -------- runShutdownCBs --------- //
	mtx.Lock()                       // Lock the mutex to protect the slice.
	defer mtx.Unlock()               // Unlock the mutex when done.
	for _, cb := range shutdownCBs { // For each callback in the slice.
		safeCall(cb) // Call the callback function.
	} // Done calling the callbacks.
} // -------- runShutdownCBs --------- //
// ------------------------------------- //
// InvokeShutdownCBs is a helper function that runs all registered shutdown
// callback functions in the order they were registered and just wraps around
// runShutdownCBs. Useful for doing a teardown without having to rely on the
// signal handler to do it for us.
// ------------------------------------ //
func InvokeShutdownCBs() { // ----- InvokeShutdownCBs -------- //
	runShutdownCBs() // Run the shutdown callbacks.
} // ------- InvokeShutdownCBs -------- //
// ------------------------------------- //
// safeCall is a helper function that executes a shutdown callback function
// with panic recovery.
// ------------------------------------- //
func safeCall(cb func()) { // ---------- safeCall ------------- //
	defer func() { // Defer the recovery function to handle panics.
		if r := recover(); r != nil { // Did we panic?
			log.Err("Recovered from panic in shutdown callback: %v", r) // Yes, log it.
		} // Done checking for panic.
	}() // Done deferring the recovery function.
	log.Inf("Running shutdown callback: %p", cb) // Log the callback.
	cb()                                         // Call the callback function.
} // ---------- safeCall ------------- //
