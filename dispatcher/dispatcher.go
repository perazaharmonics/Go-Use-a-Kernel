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

// safeCloseWrite closes the write half of a TCP connection if possible.
func safeCloseWrite(c *net.TCPConn){
  if c!=nil { _=c.CloseWrite()}
}

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
	run:=func(fn func(*net.TCPConn,*net.TCPConn,*ServerConfig,AddBytes) error,src,dst *net.TCPConn){
	  // Defer waiting for the goroutine to finish..
		defer wg.Done()                     // ..when the stack frame unwinds. 
		if err:=fn(a,b,scfg,add);err!=nil{
		  errch<-err                        // If we got an error, send it to the error channel.
		}                                   // Done defining the function.
	  safeCloseWrite(b)                   // Close the write half of the destination connection.
	}                                     // Done defining the function to run.
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
		  go run(zeroCopySend,b,a)            // Run spliceCopy on RCV.
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
scfg *ServerConfig,                             // The size of the buffer to use.
add AddBytes) error{                    // Function to add bytes to the counter.
  buffer:=make([]byte,4*scfg.BufSize)         // Create a buffer of size 256KiB.
	n,err:=io.CopyBuffer(dst,src,buffer)  // Copy the data from src to dst using the buffer.
	add(uint64(n))
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
scfg *ServerConfig,                     // The server config with the pipe capacity.
add AddBytes) error{                    // Function to add bytes to the counter.                
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
	p,err:=pipe.NewPipe2(unix.O_CLOEXEC|unix.O_NONBLOCK)    // Create a new pipe with CLOEXEC flag.
	if err!=nil{ return err }             // Error creating pipe object?	
	defer p.Close()                       // Close the pipe when done.
	_,err=p.SetCapacity(4*scfg.BufSize)         // Set the capacity of the pipe to 4*bufsiz.
	const smode=unix.SPLICE_F_MOVE|unix.SPLICE_F_MORE|unix.SPLICE_F_GIFT
	rfd:=p.GetReadEndFD()                 // The read end of the pipe.
	wfd:=p.GetWriteEndFD()                // The write end of the pipe.
	for{                                  // Unit we get EOF...
	// ---------------------------------- //
	// We will first splice and read from the source socket into the write
	// end of the pipe. Go's TCP socket is non-blocking so we will
	// have to wait for the socket to be readable before we can splice
	// ---------------------------------- //
	  n,rerr:=unix.Splice(fd0,nil,wfd,nil,scfg.BufSize,smode)
    switch{                             // Switch according to the error.
		  case rerr==syscall.EAGAIN:        // EAGAIN error?
			  // Wait for the source socket to be readable.
				if err:=waitReadable(fd0,scfg.Timeout);err!=nil{
				  return err                    // Yes, we got an error waiting for readable.
				}                               // Done waiting for readable.
		    continue                        // Yes, continue to splice.
		  case rerr!=nil:                   // Error splicing from src to pipe?
		    return err                      // Yes, return the error.
			case n==0:                        // EOF?
			  return nil                      // Yes source closed, return nil.
		}                                   // Done acting according to error code.
		// -------------------------------- //
		// Now we splice from the read end of the pipe and write into the destination
		// socket. We will do this until we have no more bytes to splice.
		// -------------------------------- //
    remaining:=n                        // Remaining bytes to splice.
		for remaining>0{                    // While we have bytes to splice...
		  m,werr:=unix.Splice(rfd,nil,fd1,nil,int(remaining),smode)
			if werr==syscall.EAGAIN{          // EAGAIN error?
			  // Wait for the destination socket to be writable.
			  if err:=waitWritable(fd1,scfg.Timeout);err!=nil{
				  return err                    // Yes, we got an error waiting for writable.
				}                               // Done waiting for writable.
			  continue                        // Yes, continue to splice.
			}                                 // Done checking for EAGAIN error.
			if werr==unix.EPIPE||werr==io.ErrClosedPipe{// Was the pipe closed?
			  return nil                      // That means we are done, return nil.
			}                                 // Done checking for closed pipe.
			if werr!=nil{                     // Error splicing from pipe to dst?
			  return err                      // Yes, return the error.
			}                                 // Done checking for error splicing.
			remaining-=m                      // We processed m bytes.
			add(uint64(m))                    // Add the bytes to the counter.
		}                                   // Done splicing the data.
	}                                     // Done splicing the data.
}                                       // ----------- spliceCopy ----------- //
// ------------------------------------ //
// zeroCopySend() uses MSG_ZEROCPY to send the data from src to dst.
// It uses the SetsockoptInt() syscall to set the MSG_ZEROCOPY flag.
// It returns the first non-EOF error if any were to occur.
// NOTE: This method seems to be the fastest, but it relies on the system being
// UNIX based and the kernel having support for MSG_ZEROCOPY.
// ------------------------------------ //
func zeroCopySend(                      // ---------- zeroCopySend ---------- //
src,dst *net.TCPConn,                   // The src and dst TCP connections.
scfg *ServerConfig,                     // The server config with the pipe capacity.
add AddBytes) error{                    // Function to add bytes to the counter.
  // ---------------------------------- //
	// File descriptor and kernel setup.
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
	if err:=setSockOptInt(fd1,unix.SOL_SOCKET,unix.SO_ZEROCOPY,1);err!=nil{
	  return err                          // Yes, return the error.
	}                                     // Done checking for error setting sock opt.
	bufMem:=make([]byte,4*scfg.BufSize)         // Create a buffer of size bufsiz.
	var seq uint32                        // Sequence number for the MSG_ZEROCOPY flag.
	var inFlight sync.Map                 // Map to track the in-flight messages.
	done:=make(chan struct{},1)           // Channel to signal when we are done.
  // ---------------------------------- //
	// Reaper loop to harvers MSG_ERRQUEUE events.
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
			  time.Sleep(50*time.Microsecond) // Yes, sleep a bit..
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
			  if e:=parseSockExtErr(cmsg);e!=nil&&e.Origin==unix.SO_EE_ORIGIN_ZEROCOPY{
				  if v,ok:=inFlight.LoadAndDelete(e.Info);ok{// Any entry in inFlight map?
						add(uint64(v.(int)))        // Yes, dequeue and add the bytes to the counter.
					}                             // Done checking for entry in inFlight map.
					if extra:=e.Data;extra>0{     // Is there an extra data?
					  seqDone:=e.Info-uint32(extra) // Sequence number done.
						for s:=seqDone+1;s<e.Info;s++{// For each sequence number done...
						  if v,ok:=inFlight.LoadAndDelete(s);ok{ // Is there an entry in the inFlight map?
							  add(uint64(v.(int)))    // Yes, dequeue and add the bytes to the counter.
							}                         // Done checking for entry in inFlight map.
						}                           // Done iterating over sequence numbers.
					}                             // Done checking for extra data.
				}                               // Done checking for extended error.    
			}                                 // Done parsing the control message.                                
		}                                   // Done listening for completion events.
		close(done)                         // Close the done channel.
	}()                                   // Done spawning the reaper thread.
	// ---------------------------------- //
	// Copy loop - read from src, zero-copy send to dst.
	// ---------------------------------- //
	// We will use the sendmsg syscall to send the data from src to dst.
	// We will use the MSG_ZEROCOPY flag to send the data without copying it
	// and the MSG_DONTWAIT flag to make it non-blocking.
	// ---------------------------------- //
readLoop:
  for{                                  // Until we get EOF or error...
	  n,rerr:=unix.Read(fd0,bufMem)       // Read data from the source connection.
		switch{                             // Switch according to the error.
		  case rerr==unix.EAGAIN||rerr==unix.EWOULDBLOCK:// EAGAIN or EWOULDBLOCK error?
		    // Wait for the source socket to be readable.
				if err:=waitReadable(fd0,scfg.Timeout);err!=nil{
				  return err                    // Yes, we got an error waiting for readable.
				}                               // Done waiting for readable.
		  case rerr!=nil&&rerr!=io.EOF:     // Any other error and NOT EOF?
			  return rerr                     // Yes, return the error.
		}                                   // Done checking for errors reading from src.
		if n>0{                             // Was any data read?
		  curr:=atomic.AddUint32(&seq,1)    // Increment the sequence number.
			inFlight.Store(curr,n)            // Enqueue # of byte for this sequence.
		  for off:=0;off<n;{                // While there are bytes to send...
			  m,serr:=unix.SendmsgN(fd1,bufMem[off:n],nil,nil,
				  unix.MSG_ZEROCOPY|unix.MSG_DONTWAIT)
				if serr==unix.EAGAIN{           // EAGAIN error?
			  // Wait for the destination socket to be writable.
			  if err:=waitWritable(fd1,scfg.Timeout);err!=nil{
				  return err                    // Yes, we got an error waiting for writable.
				}                               // Done waiting for writable.
				  continue                      // Continue and try again.
				}                               // Done checking for EAGAIN.
				if serr!=nil{                   // We actually got a send error?
				  return serr                   // Yes return the error.
				}                               // Done checking for send error.
				off+=m                          // We sent m bytes, so we move the offset.
			}                                 // Done writing to dst socket.
		}                                   // Done checking if we read any data.
		if rerr==io.EOF||n==0{              // EOF or no data read (...EOF)?
		  break readLoop                    // Yes we are done reading, so break.
		}                                   // Done checking for EOF.
	}                                     // Done waiting for data to read from src.
	// ---------------------------------- //
	// Drain loop - finish outstanding sends, keep RX path alive.
	// ---------------------------------- //
	// Now we wait for kernel to confirm all outstanding messages by listening
	// on the done channel. This will block until all messages are confirmed.
	// ---------------------------------- //
	// Close just the write end of the socket to signal peer we are done sending.
	// This will allow the reaper to exit when all messages are confirmed.
	// ---------------------------------- //
	_=unix.Shutdown(fd1,unix.SHUT_WR)     // Shutdown the write end of the socket.
	for{                                  // Until we are not in flight or we get a signal...
	  if empty:=isEmpty(&inFlight);empty{ // Is our in flight map empty?
		  return nil                        // That's great we are done, break.
		}                                   // Done checking for empty map.
		select{                             // Chose between done signal and timeout.
		  case <-done:                      // Is that a signal from the reaper?
		    return nil                      // Reaper exited, socket closed.
		  case <-time.After(200*time.Microsecond): // Timeout? No-op, just continue.
		}                                   // Done listening for signals.                
	}                                     // Done with for loop.
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
fd int,                                 // The file descriptor to wait for.
to time.Duration) error{                // How long to wait for.
  // Poll the file descriptor for readability.
  if err:=poll(fd,unix.POLLIN,to);err!=nil{// Error polling?
	  return err                          // Yes, return the error.
	}                                     // Done checking for error polling.
	return nil                            // No error, return nil.
}                                       // ---------- waitReadable ---------- //
// ------------------------------------ //
// waitWritable() waits for the file descriptor to be writable.
// It uses the poll() syscall to wait for the file descriptor to be writable.
// It returns when the file descriptor is writable.
// ------------------------------------ //
func waitWritable(                      // ---------- waitWritable ---------- //
fd int,                                 // The file descriptor to wait for.
to time.Duration) error{                // How long to wait for.          
  // Poll the file descriptor for writability.
  if err:=poll(fd,unix.POLLOUT,to);err!=nil{// Error polling?
	  return err                          // Yes, return the error.
	}                                     // Done checking for error polling.
	return nil                            // No error, return nil.
}                                       // ---------- waitWritable ---------- //
// ------------------------------------ //
// poll() waits for the file descriptor to be readable or writable.
// It uses the poll() syscall to wait for the file descriptor to be readable
// or writable. It returns when the file descriptor is readable or writable.
// ------------------------------------ //
func poll(                              // ------------- poll --------------- //
fd int,                                 // The file descriptor to wait for.
e int16,                                // The event to wait for.
d time.Duration) error{                       // How long to wait for the event.       
	t:=-1                                 // Timeout value for the poll syscall.
	if d>0{                               // Is the duration greater than 0?
	  t=int(d.Milliseconds())             // Yes, set the timeout value to the duration in milliseconds.
	}                                     // Done checking for duration.
	// Poll the file descriptor.          // This will block until the file descriptor is readable or writable.
	n,err:=unix.Poll([]unix.PollFd{{Fd: int32(fd),Events: e}},t)// readable or writable.
  if err!=nil { return err }            // Error polling the file descriptor?
	if n==0{                              // Did we timeout?
	  return syscall.ETIMEDOUT            // Yes, return ETIMEDOUT error.
	}                                     // Done checking for timeout.
	return nil                            // No error, return nil.
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
