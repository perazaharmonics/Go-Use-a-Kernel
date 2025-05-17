//go:build linux && amd64
// +build linux,amd64

// Filename: sys_pipe_linux_amd64.go
// Package pipe provides a thin wrapper around the pipe(2), pipe2(2) and mkfifo mknod syscalls.
package pipe

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	// Re-export the flags for pipe2():
	O_NONBLOCK = unix.O_NONBLOCK
	O_CLOEXEC  = unix.O_CLOEXEC
	// Re-export the fcntl pipe sizing commands:
	F_GETPIPE_SZ = unix.F_GETPIPE_SZ
	F_SETPIPE_SZ = unix.F_SETPIPE_SZ
	// Re-export the ioctl request flag for FIONREAD:
	FIONREAD = 0x541B // FIONREAD/TIOCINQ request flag value.
	// Popen read and write flags:
	POPENREAD=0      
	POPENWRITE=1
)

// Pipe is a wrapper around the pipe(2) syscall.
// It returns r, w file descriptors, or an error.
func Pipe() (r, w int, err error) {
	// The kernel expects an array of two ints (32-bit on amd64).
	var fds [2]int32
	_, _, e := unix.Syscall(unix.SYS_PIPE,
		uintptr(unsafe.Pointer(&fds)), 0, 0,
	)
	if e != 0 {
		return 0, 0, e
	}
	return int(fds[0]), int(fds[1]), nil
}

// Pipe2 is a wrapper around the pipe2(2) syscall.
// Flags can be O_NONBLOCK|O_CLOEXEC, etc.
func Pipe2(flags int) (r, w int, err error) {
	var fds [2]int32
	_, _, e := unix.Syscall(unix.SYS_PIPE2,
		uintptr(unsafe.Pointer(&fds)),
		uintptr(flags),
		0,
	)
	if e != 0 {
		return 0, 0, e
	}
	return int(fds[0]), int(fds[1]), nil
}

// Mkfifo creates a named pipe (FIFO) at the given path with the specified mode.
// Internally it invokes mknod(2) with S_IFIFO|mode.
func Mkfifo(path string, mode uint32) error {
	// Convert Go string to *byte for Syscall
	p, err := unix.BytePtrFromString(path)
	if err != nil {
		return err
	}
	// Use mknod(2): third argument is (S_IFIFO | mode), fourth is dev (zero for FIFOs)
	_, _, e := unix.Syscall(unix.SYS_MKNOD,
		uintptr(unsafe.Pointer(p)),
		uintptr(unix.S_IFIFO|mode),
		0,
	)
	if e != 0 {
		return e
	}
	return nil
}

// GetPipeSize returns the current capacity (in bytes) of the pipe referred to by fd.
func GetPipeSize(fd int) (int, error) {
	r, _, e := unix.Syscall(
		unix.SYS_FCNTL,
		uintptr(fd),
		uintptr(F_GETPIPE_SZ),
		0,
	)
	if e != 0 {
		return 0, e
	}
	return int(r), nil
}

// SetPipeSize attempts to change the capacity of the pipe referred to by fd to 'sz'.
// It returns the (possibly adjusted) new capacity.
func SetPipeSize(fd int, sz int) (int, error) {
	r, _, e := unix.Syscall(
		unix.SYS_FCNTL,
		uintptr(fd),
		uintptr(F_SETPIPE_SZ),
		uintptr(sz),
	)
	if e != 0 {
		return 0, e
	}
	return int(r), nil
}

// GetAvailableBytes is a wrapper around the ioctl(fd,FIONREAD,&cnt) syscall to
// the number of unread bytes in the pipe.
// AvailableBytes calls ioctl(fd, FIONREAD, &cnt) and returns cnt.
func GetAvailableBytes(fd int) (int, error) {
	n, e := unix.IoctlGetInt(fd, FIONREAD)
	if e != nil {
		return 0, e
	}
	return n, nil
}
// Dup is a wrapper around the dup() syscall.
func Dup(oldfd int) (int, error) {
  r,_,e:=unix.Syscall(unix.SYS_DUP,uintptr(oldfd),0,0)
  if e!=0{                              // syscall failed?
	return 0,e                          // Yes, return 0 and error.
  }                                     // No, return the new fd and nil.
  return int(r),nil                     // Return the new fd and nil.
}

// Dup2 is a wrapper around the dup2() syscall.
func Dup2(oldfd, newfd int) (int, error) {
  r,_,e:=unix.Syscall(unix.SYS_DUP2,uintptr(oldfd),uintptr(newfd),0)
  if e!=0{                              // syscall failed?
    return 0,e                          // Yes, return 0 and error.
  }                                     // No, return the new fd and nil.
  return int(r),nil                     // Return the new fd and nil.
}                                       // end of Dup2
// Dup3 is a wrapper around the dup3() syscall.
func Dup3(oldfd, newfd, flags int) (int, error) {
  r,_,e:=unix.Syscall(unix.SYS_DUP3,uintptr(oldfd),uintptr(newfd),uintptr(flags))
  if e!=0{                              // syscall failed?
	return 0,e                          // Yes, return 0 and error.
  }                                     // No, return the new fd and nil.
  return int(r),nil                     // Return the new fd and nil.
}
// Popen is similar to C's popen("cmd",mode). It creates a pipe
// then forks. In the child it hooks up pipe -> stdin/stdout, then 
// execve("/bin/sh","-c",cmd). In the parent it closes the unused end
// of the pipe. Flags must be 0, POPENREAD or POPENWRITE.
func Popen(cmd string, flags int) (fd, pid int, err error) {
  // ---------------------------------- //
  // First create a pipe
  // ---------------------------------- //
  var fds [2]int32                      // Our file descriptor set.
  if _,_,e:=unix.Syscall(unix.SYS_PIPE2,uintptr(unsafe.Pointer(&fds)),0,0);e!=0{
    return 0,0,e                        // Pipe creation failed.
  }                                     // Pipe created.
  // ---------------------------------- //
  // Fork the process
  // ---------------------------------- //
  pidraw,_,errno:=unix.Syscall(unix.SYS_FORK,0,0,0)
  if errno!=0{                          // Fork failed?
	unix.Close(int(fds[0]))             // Yes, close the pipe.
	unix.Close(int(fds[1]))             // Close the other end.
	return 0,0,errno                    // Yes, return 0 and error.
  }                                     // Done checking error
  pid=int(pidraw)                       // Get the pid
  if pid==0{                            // Are we the child process.
    if flags==POPENREAD{                // Yes, we are the child and we writing.
	  // ------------------------------ //
	  // Child writes into pipe -> Dup2(fds[1],STDOUT_FILENO)
	  // ------------------------------ //
      unix.Close(int(fds[0]))           // Close the read end of the pipe.
	  unix.Dup2(int(fds[1]),int(unix.Stdout)) // Redirect stdout to pipe.
	} else{                             // We are the child and we reading.
      // ------------------------------ //
	  // Child reads from pipe -> Dup2(fds[0],STDIN_FILENO)
	  // ------------------------------ //
	  unix.Close(int(fds[1]))           // Close the write end of the pipe.
	  unix.Dup2(int(fds[0]),int(unix.Stdin)) // Redirect stdin to pipe.
	}                                   // Done acting according to pid.
	arg0 := "sh"                        // Get the first argument.
	arg1 := "-c"                        // Get the second argument.
	argv := []string{arg0, arg1, cmd}   // Create the argument list as []string.
	env := os.Environ()                 // Get the environment as []string.
	// -------------------------------- //
	// Now execve the command.
	// -------------------------------- //
	unix.Exec("/bin/sh", argv, env)     // Execute the command with []string env.
	return -1,pid,unix.EINVAL              // Execve failed, return 0 and error.
  }                                     // Done checking pid.
  // ---------------------------------- //
  // Parent process
  // ---------------------------------- //
  if flags==POPENREAD{                  // We are the parent and we reading.
	unix.Close(int(fds[1]))             // Close the write end of the pipe.
	return int(fds[0]),pid,nil          // Return the read end of the pipe.
  }                                     // Done checking if reading.
  unix.Close(int(fds[0]))               // Close the read end of the pipe.
  return int(fds[1]),pid,nil            // Return the write end of the pipe.
}                                       // ------------ Popen ------------

// PClose waits for child pid to exit and returns its exit status.
func Pclose(pid int) (int,error){
  var ws unix.WaitStatus                // Create a wait status variable.
  _,err:=unix.Wait4(pid,&ws,0,nil)      // Wait for the child to exit.
  if err!=nil{                          // Wait failed?
    return -1,err                       // Yes, return -1 and error.
  }                                     // Done checking error.
  return ws.ExitStatus(),nil            // Return the exit status and nil.
}                                       // ----------- Pclose -----------
