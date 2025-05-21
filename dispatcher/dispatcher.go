/*============================================================================
* Filename: dispatcher.go
* 
* Description: This file contains the dispatcher functions for the proxy server.
* It contains the functions to copy data from one connection to another using
* different algorithms. The algorithms used are:
* 1. LazyCopy: This algorithm copies data from the source connection to a user
*    space buffer and then copies that data to the destination connection.
* 2. SpliceCopy: This algorithm uses the splice system call to copy data from
*    the source connection to the destination connection. It creates a pipe
*    and splices the data from the source to the write end of the pipe. Then
*    it splices the data from the read end of the pipe to the destination
*    connection.
* 3. ZeroSend: This algorithm uses the MSG_ZEROCOPY flag to send the data
*    from the source connection to the destination connection. It uses the
*    SetsockoptInt() syscall to set the MSG_ZEROCOPY flag. Then uses the
*    SendmsgN() syscall to send the data from the source connection to the
*    destination connection. It uses the recvmsg syscall to get the
*    completion events and update the inFlight map with the number of bytes
*    sent. It uses the ParseSocketControlMessage() syscall to parse the
*    socket control message to get the completion events. It uses the
*    SockExtendedErr struct to get the extended error message. Finally it
*    uses the isEmpty() function to check if the inFlight map is empty.
*
* Author:
*  J.EP J. Enrique Peraza
*=============================================================================*/

package dispatcher
import(
  "io"
	"net"
	"syscall"
	"unsafe"
	"sync"	
	"sync/atomic"
  "time"
	"golang.org/x/sys/unix"
	"github.com/ljt/ProxyServer/internal/pipe"
)

// ------------------------------------ //
// CopyPair() starts two goroutine for src<->dst using ServerConfig.Mode.
// then wait until both directions are done. It returns the first non-EOF error
// if any were to occur. This is our dispatcher.
// ------------------------------------ //
func CopyPair(a,b *net.TCPConn, scfg *ServerConfig, add AddBytes) error{
  var wg sync.WaitGroup                 // Waitgroup for goroutines to finish.
	errch:=make(chan error,2)             // To listen on errors from goroutines.
	// ---------------------------------- //
	// Ad hoc function with an anonymous function to run the dispatched alogirthm.
	// ---------------------------------- //
	run:=func(fn func(*net.TCPConn,*net.TCPConn,int,AddBytes) error,src,dst *net.TCPConn){
	  // Defer waiting for the goroutine to finish..
		defer wg.Done()                     // ..when the stack frame unwinds. 
		errch<-fn(src,dst,scfg.BufSize,add) // Run and send the error to errch.
	}                                     // Done defining the function.
	// ---------------------------------- //
	// Now we add two units to the waitgroup, and according to the mode
	// we run the function with the right algorithm.
	// ---------------------------------- //
	wg.Add(2)                             // Add two units to the waitgroup.
	switch scfg.Mode{                     // Act according to the mode.
	  case LazyCopy:                      // We are using LazyCopy.
		  go run(lazyCopy,a,b)              // Run the function with LazyCopy.
			go run(lazyCopy,b,a)              // Run the function with LazyCopy.
		case SpliceCopy:                    // We are using SpliceCopy.
	    go run(spliceCopy,a,b)						// Run the function with SpliceCopy.
			go run(spliceCopy,b,a)						// Run the function with SpliceCopy.
		case ZeroSend:                      // We are using ZeroSend.
		  go run(zeroCopySend,a,b)          // Run zeroCopySend on XMT.
		  go run(spliceCopy,b,a)            // Run spliceCopy on RCV.
		default:                            // We are using the default.
		 go run(spliceCopy,a,b)             // Run the function with LazyCopy.
		 go run(spliceCopy,b,a)             // Run the function with LazyCopy. 
	}                                     // Done dispatching according to mode.
	wg.Wait()                             // Wait for the goroutines to finish.
	close(errch)                          // Close the error channel.
	// ---------------------------------- //
	// Now we check for errors in the error channel.
	// ---------------------------------- //
	for err:=range errch{                 // For each error channel...
	  if err!=nil&&err!=io.EOF{           // Error and is not EOF in THIS channel?
		  return err                        // Return the error.
		}                                   // Done checking error in THIS channel.
	}                                     // Done listening to the error channel.
	return nil                            // Return nil if no errors were found.                                    
}
// ------------------------------------ //
// lazyCopy() Copies data from src into a user space buffer then copies that
// data to dst. It returns the first non-EOF error if any were to occur.
// ------------------------------------ //
func lazyCopy(                          // ------------ lazyCopy ------------ //
src,dst *net.TCPConn,                   // The src and dst TCP connections.
bufsiz int,                             // The size of the buffer to use.
add AddBytes) error{              // Function to add bytes to the counter.
  buffer:=make([]byte,bufsiz)           // Create a buffer of size bufsiz.
	n,err:=io.CopyBuffer(dst,src,buffer)  // Copy the data from src to dst using the buffer.
	add(int(n))
	return err                            // Return the error if any.
}                                       // ------------ lazyCopy ------------ //
// ------------------------------------ //
// spliceCopy() uses the splice system call to copy data from src to dst.
// It creates a pipe and splices the data from the src to the write end of the
// pipe. Then it splices the data from the read end of the pipe to the dst.
// It returns the first non-EOF error if any were to occur.
// ------------------------------------ //
func spliceCopy(                        // ----------- spliceCopy ----------- //
src, dst *net.TCPConn,                  // The src and dst TCP connections.
bufsiz int,                             // The size of the buffer to use.
add AddBytes) error{              // Function to add bytes to the counter.                
  fd0,err:=connFD(src)                  // The fd of the source connection.
	if err!=nil{                          // Error getting the sock fd?
		return err                          // Yes, return the error.
	}                                     // Done checking for error geting fd.
	fd1,err:=connFD(dst)                  // The fd of the destination connection.
	if err!=nil{                          // Error getting the sock fd?
		return err                          // Yes, return the error.
	}                                     // Done checking for error geting fd.
  // ---------------------------------- //
	// Create a pipe to splice the data from src to dst.
	// ---------------------------------- //
	p,_:=pipe.NewPipe2(unix.O_CLOEXEC)    // Create a new pipe with CLOEXEC flag.
	const smode=unix.SPLICE_F_MOVE|unix.SPLICE_F_MORE// Bit flags for splice.
	rfd:=p.GetReadEndFD()                 // The read end of the pipe.
	wfd:=p.GetWriteEndFD()                // The write end of the pipe.
	for{                                  // Unit we get EOF...
	// ---------------------------------- //
	// We will first splice and read from the source socket into the write
	// end of the pipe. Go's TCP socket is non-blocking so we will
	// have to wait for the socket to be readable before we can splice
	// ---------------------------------- //
	  n,err:=unix.Splice(fd0,nil,wfd,nil,bufsiz,smode)
    switch{                             // Switch according to the error.
		  case err==syscall.EAGAIN:         // EAGAIN error?
			  waitReadable(fd0)               // Wait for the source socket to be readable.
		    continue                        // Yes, continue to splice.
		  case err!=nil:                    // Error splicing from src to pipe?
		    return err                      // Yes, return the error.
			case n==0:                        // EOF?
			  return nil                      // Yes source closed, return nil.
		}                                   // Done acting according to error code.
		add(int(n))                         // Add the number of bytes transferred.
		// -------------------------------- //
		// Now we splice from the read end of the pipe and write into the destination
		// socket. We will do this until we have no more bytes to splice.
		// -------------------------------- //
    remaining:=n                        // Remaining bytes to splice.
		for remaining>0{                    // While we have bytes to splice...
		  m,err:=unix.Splice(rfd,nil,fd1,nil,int(remaining),smode)
			if err==syscall.EAGAIN{           // EAGAIN error?
			  waitWritable(fd1)               // Wait for the destination socket to be writable.
			  continue                        // Yes, continue to splice.
			}                                 // Done checking for EAGAIN error.
			if err==unix.EPIPE||err==io.ErrClosedPipe{// Was the pipe closed?
			  return nil                      // That means we are done, return nil.
			}                                 // Done checking for closed pipe.
			if err!=nil{                      // Error splicing from pipe to dst?
			  return err                      // Yes, return the error.
			}                                 // Done checking for error splicing.
			remaining-=m                      // We processed m bytes.
		}                                   // Done splicing the data.
	}                                     // Done splicing the data.
}                                       // ----------- spliceCopy ----------- //
// ------------------------------------ //
// zeroCopySend() uses MSG_ZEROCPY to send the data from src to dst.
// It uses the SetsockoptInt() syscall to set the MSG_ZEROCOPY flag.
// It returns the first non-EOF error if any were to occur.
// ------------------------------------ //
func zeroCopySend(                      // ---------- zeroCopySend ---------- //
src,dst *net.TCPConn,                   // The src and dst TCP connections.
bufsiz int,                             // The size of the buffer to use.
add AddBytes) error{              // Function to add bytes to the counter.
  // ---------------------------------- //
	// Use the SetsockoptInt() syscall to set the MSG_ZEROCOPY flag. This allows
	// us to send the data from src to dst without copying it to user space.
	// ---------------------------------- //
	fd0,err:=connFD(src)                  // The fd of the source connection.
	if err!=nil{                          // Error getting the sock fd?
	  return err                          // Yes, return the error.
	}                                     // Done checking for error geting fd.
	fd1,err:=connFD(dst)                  // The fd of the destination connection.
	if err!=nil{                          // Error getting the sock fd?
	  return err                          // Yes, return the error.
	}                                     // Done checking for error geting fd.
	if err:=setSockOptInt(fd1,unix.SOL_SOCKET,unix.SO_SNDBUF,1);err!=nil{
	  return err                          // Yes, return the error.
	}                                     // Done checking for error setting sock opt.
	bufMem:=make([]byte,bufsiz)           // Create a buffer of size bufsiz.
	var seq uint32                        // Sequence number for the MSG_ZEROCOPY flag.
	var inFlight sync.Map                 // Map to track the in-flight messages.
	done:=make(chan struct{},1)           // Channel to signal when we are done.
  // ---------------------------------- //
	// Spawn a goroutine to reap completion events.
	// ---------------------------------- //
	go func(){                            // On a spearate thread...
	  ctl:=make([]byte,256)               // Control buffer for the recvmsg syscall.
	  // -------------------------------- //
		// This is our reaper thread. It will listen for completion events
		// and update the inFlight map with the number of bytes sent, extract that
		// data and add it to the counter.
		// -------------------------------- //
		for{                                // Until we close the done channel...
		  // ------------------------------ //
			// Use the recvmsg syscall to get the completion events.
			// Use the MSG_ERRQUEUE flag to get the completion events.
			// ... and use the MSG_DONTWAIT flag to make it non-blocking.
			// ------------------------------ //
			_,oobn,_,_,err:=unix.Recvmsg(int(fd1),nil,ctl,
			  unix.MSG_ERRQUEUE|unix.MSG_DONTWAIT)// Get the completion events.
			if err == unix.EAGAIN{            // Did we get EAGAIN?
			  time.Sleep(200*time.Microsecond)// Yes, sleep a bit..
				continue                        // ... and continue.
			}                                 // Done checking for EAGAIN error.
			if err!=nil{                      // Did we get an error rcving the msg?
			  break                           // Yes, we cant listen anymore, break.
			}                                 // Done checking for error rcving msg.
			// ------------------------------ //
			// Parse the socket control message to get the completion events,
			// Which are placed in the ctrl buffer
      // ------------------------------ //
			msgs,_:=unix.ParseSocketControlMessage(ctl[:oobn])
			for _,cmsg:=range msgs{           // For each control message...
			  exterr:=parseSockExtErr(cmsg)
				// Not a extended error message?
				if exterr==nil||exterr.Origin!=unix.SO_EE_ORIGIN_ZEROCOPY{
				  continue                      // No, continue to the next message.
				}                               // Done checking for extended error message.
				if val,ok:=inFlight.Load(exterr.Info);ok{// Any info in the map?
				  add(int(val.(int)))      // Yes, add the bytes to the counter.
					inFlight.Delete(exterr.Info)  // Remove the entry from the map.
				}                               // Done checking for info in the map.
			}                                 // Done parsing the control message.                                
		}                                   // Done listening for completion events.
		close(done)                         // Close the done channel.
	}()                                   // Done spawning the reaper thread.
	// ---------------------------------- //
	// We will use the sendmsg syscall to send the data from src to dst.
	// We will use the MSG_ZEROCOPY flag to send the data without copying it
	// and the MSG_DONTWAIT flag to make it non-blocking.
	// ---------------------------------- //
	for{                                  // Until we get EOF...
	  n,rderr:=unix.Read(fd0,bufMem)      // Place data from src into the buffer.
		if n==0&&rderr==nil{                // EOF and no error?
		  break                             // Yes we are done, break from the loop.
		}                                   // Done checking for EOF.
		if rderr!=nil&&rderr!=io.EOF{       // Error reading from src?
		  return rderr                      // Yes, return the error.
		}                                   // Done checking for error reading.
		seq:=atomic.AddUint32(&seq,1)       // Increment the sequence number.
		inFlight.Store(seq,n)               // In this sequence we have n bytes.
		// -------------------------------- //
		// Now we will send the data from src to dst using the sendmsg syscall.
		// -------------------------------- //
		for off:=0;off<n;{       // While we have bytes to send...
			m,err:=unix.SendmsgN(fd1,bufMem[off:n],nil,nil,
			  unix.MSG_ZEROCOPY|unix.MSG_DONTWAIT)// Send the data in buffer to dst.
			if err==unix.EAGAIN{              // EAGAIN error?
			  waitWritable(fd1)               // Wait for dst socket to be writable.
		    continue                        // Yes, continue to send.
		  }                                 // Done checking for EAGAIN error.
			off+=m                            // We sent m bytes, so we need to send the rest.                     
	  }                                   // Done sending the data.
		if rderr==io.EOF{                   // Did we get EOF from src?
		  break                             // That's good, break from the loop.
		}                                   // Done checking for EOF.
	}                                     // Done reading from src and sending to dst.
	// ---------------------------------- //
	// Now we wait for kernel to confirm all outstanding messages by listening
	// on the done channel. This will block until all messages are confirmed.
	// ---------------------------------- //
	for{                                  // Until we are not in flight or we get a signal...
	  if empty:=isEmpty(&inFlight);empty{ // Is our in flight map empty?
		  break                             // That's great we are done, break.
		}                                   // Done checking for empty map.
		select{                             // Chose between done signal and timeout.
		  case <-done:                      // Is that a signal from the reaper?
		    return nil                      // Reaper exited, socket closed.
		  case <-time.After(200*time.Microsecond): // Timeout? No-op, just continue.
		}                                   // Done listening for signals.                
	}                                     // Done with for loop.
	return nil                            // The good ending, no errors found.
}                                       // ---------- zeroCopySend ---------- //
// ------------------------------------ //
// isEmpty() checks if the inFlight map is empty.
// ------------------------------------ //
func isEmpty(
m *sync.Map) bool{                      // Our in flight memory map.
  empty:=true                           // Assume the map is empty.
	// ---------------------------------- //
	// If we can range it, then it is de-facto populated. As a range is
	// a measurement of distance. Even if there is the nothingness of 0s.
	// ---------------------------------- // 
	m.Range(func(_,_ any) bool{           // For each entry in the map...
	  empty=false                         // We have an entry, so we are not empty.
		return false                        // Break from the loop.             
	})                                    // Done checking for entries in the map.
	return empty                          // Return true if the map is empty.
}                                       // ------------ isEmpty ------------- //
// ------------------------------------ //
// connFD() returns the file descriptor of the TCP connection. It is a helper
// function that we will use to splice() the connection to the pipe.
// ------------------------------------ //
func connFD(                            // ------------- connFD ------------- //
c *net.TCPConn) (int,error){            // Get the file descriptor of the TCP connection.
	raw,err:=c.SyscallConn()              // Get the raw connection.
	if err!=nil{                          // Error getting the raw connection struct?
		return 0,err                        // Yes, return 0 and the error.
	}                                     // Done checking error getting raw socks.
  var fd int                            // File descriptor for the connection.
  if err:=raw.Control(func(u uintptr){  // Ad hoc function to get the file descriptor.
	  fd=int(u)                           // Get the file descriptor.
	}); err!=nil{                         // Error getting the file descriptor?
	  return 0, err                       // Can't do no more, return 0 and the error.
	}                                     // Done checking error getting fd.
	return fd,nil                         // Return the file descriptor and no error.
}                                       // ------------- connFD ------------- //
// ------------------------------------ //
// waitReadable() waits for the file descriptor to be readable.
// It uses the poll() syscall to wait for the file descriptor to be readable.
// It returns when the file descriptor is readable.
// ------------------------------------ //
func waitReadable(                      // ---------- waitReadable ---------- //
fd int){                                // The file descriptor to wait for.
  poll(fd,unix.POLLIN)                  // Poll the file descriptor for readability.
}                                       // ---------- waitReadable ---------- //
// ------------------------------------ //
// waitWritable() waits for the file descriptor to be writable.
// It uses the poll() syscall to wait for the file descriptor to be writable.
// It returns when the file descriptor is writable.
// ------------------------------------ //
func waitWritable(                      // ---------- waitWritable ---------- //
fd int){                                // The file descriptor to wait for.
  poll(fd,unix.POLLOUT)                 // Poll the file descriptor for writability.
}                                       // ---------- waitWritable ---------- //
// ------------------------------------ //
// poll() waits for the file descriptor to be readable or writable.
// It uses the poll() syscall to wait for the file descriptor to be readable
// or writable. It returns when the file descriptor is readable or writable.
// ------------------------------------ //
func poll(                              // ------------- poll --------------- //
fd int,                                 // The file descriptor to wait for.
e int16){                               // The event to wait for.
  p:=[]unix.PollFd{{Fd: int32(fd),Events: e}}
	_,_=unix.Poll(p,-1)                   // Poll file descriptor for the event.
}                                       // ------------- poll --------------- //
// ------------------------------------ //
// SetSockOptInt() sets the socket option for the file descriptor. It is a
// wrapper for the SetsockoptInt() syscall. It sets the socket option
// for the file descriptor to the value specified.
// It returns the error if any.
// ------------------------------------ //
func setSockOptInt(                     // ---------- setSockOptInt --------- //
fd int,                                 // The file descriptor to set sock option for.
level,optname,optval int) error{        // The level, option name and value.
	if err:=unix.SetsockoptInt(fd,level,optname,optval);err!=nil{
		return err                          // Yes, return the error.
	}                                     // Done checking for error setting sock opt.
	return nil                            // Return nil if no errors were found.
}                                       // ---------- setSockOptInt --------- //
// ------------------------------------ //
// parseSockExtEr() extracts *unix,SickExtendedErr from a control message.
// ------------------------------------ //
func parseSockExtErr(
msg unix.SocketControlMessage) (*unix.SockExtendedErr){
  // compute the size of the struct at compile time
	const sizeof=unsafe.Sizeof(unix.SockExtendedErr{})
	if len(msg.Data)<int(sizeof){         // Is the data smaller than the struct?
	  return nil                          // Yes, return nil.
	}                                     // Done checking for size of data.
	return (*unix.SockExtendedErr)(unsafe.Pointer(&msg.Data[0]))
}                                       // Done extracting the extended error struct.
