//go:build linux && amd64
// +build linux,amd64

// Filename: pipe.go
// Package pipe provides high-level pipe operations (os.File based)
// on top of the low-level syscalls in sys_pipe_linux_amd64.go.
package pipe

import (
	"os"
)

type Pipes struct {
  rf   *os.File // Read end of the pipe
  wf   *os.File // Write end of the pipe
  rfd  int      // Read file descriptor
  wfd  int      // Write file descriptor
  flgs int      // Flags for pipe2
}

// NewAnonymousPipe is like os.Pipe(), but uses our shim under the hood.
// It returns the read & write ends as *os.File.
func NewPipe() (*Pipes, error) {
  rfd, wfd, err:= Pipe()                // Call the low-level pipe syscall
  if err != nil {                       // Did we error getting the pipe's fd?
    return nil, err                     // Yes, return nil object and error.
  }                                     // Done with error creating pipe.
  return &Pipes{                        // Return our pipe object.
    rf: os.NewFile(uintptr(rfd), "pipe-r"), // Create the read end of the pipe
    rfd: rfd,                           // Set the read file descriptor
    wfd: wfd,                           // Set the write file descriptor
    wf: os.NewFile(uintptr(wfd), "pipe-w"),// Create the write end of the pipe
  }, nil                                // Done creating pipe object.
}                                       // ------------ NewPipe ------------- //

// NewPipeWithFlags is like os.Pipe() + fcntl flags calls pipe2(2).
// flags is any combination of O_CLOEXEC, O_NONBLOCK, etc.
func NewPipe2(flags int) (*Pipes,error) {
  rfd,wfd,err:=Pipe2(flags)             // Call the low-level pipe2 syscall
  if err!=nil{                          // Did we error getting the pipe's fd?
    return nil,err                      // Yes, return nil object and error.
  }                                     // Done with error creating pipe.
  return &Pipes{                        // Return our pipe object.
    rf: os.NewFile(uintptr(rfd), "pipe-r"), // Create the read end of the pipe
    rfd: rfd,                           // Set the read file descriptor
    wfd: wfd,                           // Set the write file descriptor  
    wf: os.NewFile(uintptr(wfd), "pipe-w"), // Create the write end of the pipe
    flgs: flags,                        // Set the flags for the pipe
    },nil                               // Done creating pipe object.
}                                       // ------------ NewPipe2 ------------ //
// Pipe return the read and write ends of the pipe given a file descriptor set.
func Piper(fd []int32) (*Pipes,error){
  if len(fd)!=2{                        // Did they give us a valid fd set?
    return nil,os.ErrInvalid            // No, return nil and error.
  }                                     // Done checking if the fd set is valid.
  return &Pipes{                        // Return our pipe object.
  rf: os.NewFile(uintptr(fd[0]), "pipe-r"), // Create the read end of the pipe
  rfd: int(fd[0]),                      // Set the read file descriptor
  wfd: int(fd[1]),                      // Set the write file descriptor
  wf: os.NewFile(uintptr(fd[1]), "pipe-w"), // Create the write end of the pipe
  },nil                                 // Done creating pipe object.
}                                       // ------------ Piper -------------- //
// Piper2 returns the read and write ends of the pipe given a file descriptor set
// and the flags for the pipe.
func Piper2(fd []int32,flags int) (*Pipes,error){
  if len(fd)!=2{                        // Did they give us a valid fd set?
    return nil,os.ErrInvalid            // No, return nil and error.
  }                                     // Done checking if the fd set is valid.
  return &Pipes{                        // Return our pipe object.
  rf: os.NewFile(uintptr(fd[0]), "pipe-r"), // Create the read end of the pipe
  rfd: int(fd[0]),                      // Set the read file descriptor
  wfd: int(fd[1]),                      // Set the write file descriptor  
  wf: os.NewFile(uintptr(fd[1]), "pipe-w"), // Create the write end of the pipe
  flgs: flags,                          // Set the flags for the pipe
  },nil                                 // Done creating pipe object.
}                                       // ------------ Piper2 ------------- //

// GetWriteEnd returns the write end of the pipe.
func (p *Pipes) GetWriteEnd() (*os.File, error) {
  if p.wf == nil {                      // Is the write end of the pipe nil?
    return nil, os.ErrInvalid           // Yes, return nil and error
  }					// Done checking if the write end of the pipe is nil.
  return p.wf, nil                      // Return the write end of the pipe
}                                       // ------------ GetWriteEnd --------- //
// GetReadEnd returns the read end of the pipe.
func (p *Pipes) GetReadEnd() (*os.File, error) {
  if p.rf == nil{                       // Is the read end of the pipe nil?
	return nil, os.ErrInvalid       // Yes, return nil and error
  }                                     // Done checking if the read end of the pipe is nil.
  return p.rf, nil                      // Return the read end of the pipe
}                                       // ------------ GetReadEnd ---------- //
// GetReadEndFD return the read end of the pipe file descriptor.
func (p *Pipes) GetReadEndFD() int {
  if p.rf == nil{                       // Is the read end of the pipe nil?
    return -1                           // Yes, return -1
  }                                     // Done checking if read end is nil.
  return p.rfd                          // Return fd[0]
}                                       // ------------ GetReadEndFD --------- //
// GetWriteEndFD return the write end of the pipe file descriptor.
func (p *Pipes) GetWriteEndFD() int {
  if p.wf == nil{                       // Is the write end of the pipe nil?
    return -1                           // Yes, return -1
  }                                     // Done checking if write end is nil.
  return p.wfd                          // Return fd[1]
}                                       // ------------ GetWriteEndFD -------- //
// SetCapacity sets the pipe buffer size (bytes) on f.
// Returns the new (kernel-adjusted) size.
func (p *Pipes) SetCapacity(f *os.File, size int) (int, error) {
  return SetPipeSize(int(p.rf.Fd()), size)
}

// Capacity returns the current pipe buffer capacity (bytes) on f.
func (p *Pipes) Capacity(f *os.File) (int, error) {
	return GetPipeSize(int(p.wf.Fd()))
}

// Available returns the number of bytes queued in the pipe ready to read.
func (p *Pipes) Available(f *os.File) (int, error) {
	return GetAvailableBytes(int(p.rf.Fd()))
}

// Read() reads from the pipe and returns the number of bytes read.
func (p *Pipes) Read(b []byte) (int, error) {
  if p.rf == nil {                      // Is the read end of the pipe nil?
    return 0, os.ErrInvalid             // Yes, return 0 and error
  }	                                // Done checking if the read end of the pipe is nil.
  n, err := p.rf.Read(b)                // Read from the pipe
  return n, err                         // No error, return the number of bytes read and nil.
}                                       // ------------ Read ----------------- //
// Write() writes to the pipe and returns the number of bytes written.
func (p *Pipes) Write(b []byte) (int, error) {
  if p.wf==nil{                         // Is the write end of the pipe nil?
	return 0,os.ErrInvalid          // Yes, return 0 and error
  }                                     // Done checking if the write end of the pipe is nil.
  n,err:=p.wf.Write(b)                  // Write to the pipe
  return n,err                          // No error, return the number of bytes written and nil.
}                                       // ------------ Write ---------------- //

// Close closes the read and write files associated with the pipe by being given
// the read or write file descriptor.
func (p *Pipes) Close() error {
  if err:=p.rf.Close();err!=nil{            // Did we error closing the read end of the pipe?
    _=p.wf.Close()                          // Yes, close the write end of the pipe.
    return err                              // Return the error closing the read end of the pipe.
    }                                       // Done closing the read end of the pipe.
  return p.wf.Close()                       // Close the write end of the pipe.
}                                           // ------------ Close --------------- //

// CloseRead closes the read end of the pipe.
func (p *Pipes) CloseRead() error {
  if p.rf==nil{                         // Is the read end of the pipe nil?
	return nil                      // Nothing to do, return nil.
  }                                     // Done checking if the read end of the pipe is nil.
  err:=p.rf.Close()                     // Close the read end of the pipe.
  p.rf=nil                              // Set the read end of the pipe to nil.
  p.rfd=-1                              // Set read fd to -1.
  return err                            // Return the error closing the read end of the pipe.
}                                       // ------------ CloseRead ----------- //
// CloseWrite closes the write end of the pipe.
func (p *Pipes) CloseWrite() error {
  if p.wf==nil{                         // Is the write end of the pipe nil?
	return nil                      // Nothing to do, return nil.
  }                                     // Done checking if the write end of the pipe is nil.
  err:=p.wf.Close()                     // Close the write end of the pipe.
  p.wf=nil                              // Set the write end of the pipe to nil.
  p.wfd=-1                              // Set write end fd to -1.
  return err                            // Return the error closing the write end of the pipe.
}                                       // ------------ CloseWrite ---------- //
// DupFile duplicates fs descriptor (using SYS_DUP) and returns a new *os.File.
func DupFile(f *os.File) (*os.File,error) {
  // ---------------------------------- //
  // Create a new file with the lowest available file descriptor.
  // ---------------------------------- //
  if f==nil{                            // Did they give us a file
    return nil,os.ErrInvalid            // Yes, return nil and error.
  }                                     // Done checking if the file is nil.
  oldfd:=int(f.Fd())                    // Get the file descriptor of the file.
  newfd,err:=Dup(oldfd)                 // Duplicate the file descriptor.
  if err!=nil{                          // Did we error duplicating the file descriptor?
    return nil,err                      // Yes, return nil and error.
  }                                     // Done with error duplicating the file descriptor.
  return os.NewFile(uintptr(newfd),f.Name()),nil// Return new file and nil error.
}                                       // ------------ DupFile -------------- //

// Dup2File duplicates fs descriptor (using SYS_DUP2) and returns a new *os.File.
func Dup2File(f *os.File, newfd int) (*os.File,error) {
  // ---------------------------------- //
  // Create a new file with the lowest available file descriptor.
  // ---------------------------------- //
  if f==nil{                            // Did they give us a file
    return nil,os.ErrInvalid            // Yes, return nil and error.
  }                                     // Done checking if the file is nil.
  oldfd:=int(f.Fd())                    // Get the file descriptor of the file.
  newfd,err:=Dup2(oldfd,newfd)          // Duplicate the file descriptor.
  if err!=nil{                          // Did we error duplicating the file descriptor?
    return nil,err                      // Yes, return nil and error.
  }                                     // Done with error duplicating the file descriptor.
  return os.NewFile(uintptr(newfd),f.Name()),nil// Return new file and nil error.
}                                       // ------------ Dup2File ------------- //

// Dup3File makes newfd a copy of f.Fd() with flags (e.g. O_CLOEXEC).
// closing newfd first. Flags get passed to Dup3() wrapper
func Dup3File(f *os.File, newfd int, flags int) (*os.File,error) {
  // ---------------------------------- //
  // Create a new file with the lowest available file descriptor.
  // ---------------------------------- //
  if f==nil{                            // Did they give us a file
    return nil,os.ErrInvalid            // Yes, return nil and error.
  }                                     // Done checking if the file is nil.
  oldfd:=int(f.Fd())                    // Get the file descriptor of the file.
  newfd,err:=Dup3(oldfd,newfd,flags)    // Duplicate the file descriptor.
  if err!=nil{                          // Did we error duplicating the file descriptor?
    return nil,err                      // Yes, return nil and error.
  }                                     // Done with error duplicating the file descriptor.
  return os.NewFile(uintptr(newfd),f.Name()),nil// Return new file and nil error.
}

// DupFile duplicates fs descriptor (using SYS_DUP) and returns a new *os.File.
func DupFD(fd int) (int,error) {
  // ---------------------------------- //
  // Create a new file with the lowest available file descriptor.
  // ---------------------------------- //
  oldfd:=fd                             // Get the file descriptor of the file.
  newfd,err:=Dup(oldfd)                 // Duplicate the file descriptor.
  if err!=nil{                          // Did we error duplicating the file descriptor?
    return 0,err                        // Yes, return nil and error.
  }                                     // Done with error duplicating the file descriptor.
  return newfd,nil                      // Return the new fd and nuil error.
}                                       // ------------ DupFile -------------- //

// Dup2FD duplicates a fd. It takes an old fd and a new fd. If newfd is already
// open, it will be closed first. Then newfd is made a copy of oldfd. Meaning
// that both file descriptors refer to the same open file description.
func Dup2FD(oldfd, newfd int) (int,error) {
  // ---------------------------------- //
  // Create a new file with the lowest available file descriptor.
  // ---------------------------------- //
  newfd,err:=Dup2(oldfd,newfd)          // Duplicate the file descriptor.
  if err!=nil{                          // Did we error duplicating the file descriptor?
    return 0,err                        // Yes, return nil and error.
  }                                     // Done with error duplicating the file descriptor.
  return newfd,nil                      // Return the new fd and nuil error.
}                                       // ------------ Dup2File ------------- //

// Dup3FD is the same as Dup2FD except that it takes a flag argument.
func Dup3FD(oldfd, newfd, flags int) (int,error) {
  // ---------------------------------- //
  // Create a new file with the lowest available file descriptor.
  // ---------------------------------- //
  newfd,err:=Dup3(oldfd,newfd,flags)    // Duplicate the file descriptor.
  if err!=nil{                          // Did we error duplicating the file descriptor?
    return 0,err                        // Yes, return nil and error.
  }                                     // Done with error duplicating the file descriptor.
  return newfd,nil                      // Return the new fd and nuil error.
}                                       // ----------- Dup3File ------------- //


// POpen starts 'sh -c cmd' and returns an *os.File hooked to either the child's
// stdout (in r mode) os stdin (in w mode), plus the Go *os.Process you can Wait()
// on
func POpen(cmd,mode string) (f *os.File,proc *os.Process,err error) {
  if cmd==""||mode==""{                 // Did they give us a command or mode?
    return nil,nil,os.ErrInvalid        // No, return nil and error.
  }                                     // Done checking if the command and mode are empty.
  var modes int                         // The modes we can handle.
  switch mode{                          // Act according to the mode.
  case "r":                             // We are in read mode.
    modes=POPENREAD                     // Set the modes to read.
  case "w":                             // We are in write mode.
    modes=POPENWRITE                    // Set the modes to write.
  default:                              // We are in an unknown mode.
    return nil,nil,os.ErrInvalid        // Yes, return nil and error.
  }                                     // Done checking the mode.
  // ---------------------------------- //
  // Create a pipe
  // ---------------------------------- //
  fd,pid,err:=Popen(cmd,modes)          // Call the low-level popen syscall
  if err!=nil{                          // Did we error getting the pipe's fd?
    return nil,nil,err                  // Yes, return nil object and error.
  }                                     // Done with error creating pipe.
  file:=os.NewFile(uintptr(fd),"popen-"+mode)
  // ---------------------------------- //
  // Wrap the raw pid in the *os.Process so the user can call proc.Wait()
  // ---------------------------------- //
  proc,err=os.FindProcess(pid)          // Find the process by pid.
  if err!=nil{                          // Did we error finding the process?
    file.Close()                        // Yes, close the file.
    return nil,nil,err                  // return nil object and error.
  }                                     // Done with error finding the process.
  return file,proc,nil                  // Return the file and process.
}                                       // ------------ POpen --------------- //
// PClose closes the *os.File and then waits for the process to exit, returning
// its exit code or error.
func PClose(f *os.File, proc *os.Process) (int, error) {
  // ---------------------------------- //
  // Close the fd so the child sees EOF (if writing to it) or fd is cleaned up.
  // ---------------------------------- //
  if f==nil{                            // Did they give us a file
    return 0,os.ErrInvalid              // Yes, return 0 and error.
  }                                     // Done checking if the file is nil.
  f.Close()                             // Close the file.
  // ---------------------------------- //
  // Wait for the process to exit and return its exit code.
  // ---------------------------------- //
  code,err:=Pclose(proc.Pid)            // Wait for the process to exit.
  if err!=nil{                          // Did we error waiting for the process?
    return -1,err                       // Yes, return -1 and error.
  }                                     // Done with error waiting for the process.
  return code,nil                       // No error, return the exit code and nil.
}                                       // ------------ PClose -------------- //

// CreateFIFO makes a named pipe (FIFO) at path with the given permissions.
func CreateFIFO(path string, perm os.FileMode) error {
	return Mkfifo(path, uint32(perm.Perm()))
}
// OpenFIFO opens a named pipe (FIFO) at path with the given permissions.
func OpenFIFO(path string, perm os.FileMode) (*os.File, error) {
  f,err:=os.OpenFile(path,os.O_RDWR|os.O_CREATE|os.O_EXCL,perm) // Open the FIFO
  if err!=nil{                          // Did we error opening the FIFO?
	return nil,err                  // Yes, return nil object and error.
  }                                     // Done with error opening FIFO.
  return f,nil                          // Return the FIFO object.
}                                       // ------------ OpenFIFO ------------ //
// CloseFIFO closes the named pipe (FIFO) at path.
func CloseFIFO(path string) error {
  err:=os.Remove(path)                  // Remove the FIFO
  if err!=nil{                          // Did we error removing the FIFO?
	return err                      // Yes, return the error.
  }					// Done with error removing FIFO.
  return nil                            // No error, return nil.
}                                       // ------------ CloseFIFO ----------- //
