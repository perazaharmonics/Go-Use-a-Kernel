//go:build linux && amd64
// ****************************************************************************
// Filename: sysv_linux_amd64.go
// Description: This file contains the implementation of the SysV semaphore
// as a shim that wraps around the Sys_semget(), Sys_semctl(), and Sys_semop()
// system calls, to use in the semaphore package.
//
//
// Author:
//  J.EP J. Enrique Peraza,
//
// ****************************************************************************
package semaphore
import (
  "unsafe"                              // For unsafe pointer manipulation
  "golang.org/x/sys/unix"               // For the SysV IPC constants and system calls
)

// Re-export the SysV IPC constants that were hidden in the new sys/unix package
// This is a shim to make the SysV IPC constants and system calls available in 
// the new sys/unix
const (
  GETVAL = 12                           // The SysV IPC const for get value
	SETVAL = 16                           // The SysV IPC const for set value
	IPC_RMID = unix.IPC_RMID              // The SysV IPC const for remove id
)

type sembuf struct {
	SemNum uint16
	SemOp  int16
	SemFlg int16
	_      uint16 // Padding to make the struct 8 bytes
}
// ------------------------------------ //
// semget is a wrapper for the Sys_semget() syscall
// ------------------------------------ //
func semget(key,nsems,flag int) (int, error) {
  // Sem ID, number of semaphores, and error are the return values
	semid,_,e:=unix.Syscall(unix.SYS_SEMGET,uintptr(key),uintptr(nsems),
	  uintptr(flag))                      // Call the Sys_semget syscall
	if e!=0 {                             // Did we get an error?
	  return 0, e                         // Yes, return no semid and the error
	}                                     // No error, we got a semid
	return int(semid), nil                // Return the semid and no error
}                                       // -------------- semget ------------ //
// ------------------------------------ //
// semctl is a wrapper for the Sys_semctl() syscall
// ------------------------------------ //
func semctl(id,num,cmd,arg int) (int,error){
  // Sem ID, number of semaphores, and error are the return values
	semid,_,e:=unix.Syscall6(unix.SYS_SEMCTL,uintptr(id),uintptr(num),
	  uintptr(cmd),uintptr(arg),0,0)      // Call the Sys_semctl syscall
	if e!=0 {                             // Did we get an error?
	 return 0, e                          // Yes, return no semid and the error
	}                                     // No error, continue
	return int(semid), nil                // Return the semid and no error
}                                       // -------------- semctl ------------ //
// ------------------------------------ //
// semop is a wrapper for the Sys_semop() syscall
// ------------------------------------ //
func semop(id int,sops []sembuf) error{
  if len(sops)==0{                      // Do we have no operations?
	 return nil                           // Return no error, we did nothing.
	}                                     // Done checking for no operations
	_,_,e:=unix.Syscall(unix.SYS_SEMOP,uintptr(id),uintptr(unsafe.Pointer(&sops[0])),
	  uintptr(len(sops)))                 // Call the Sys_semop syscall
	if e!=0 {                             // Did we get an error?
	 return e                             // Yes, return the error
	}                                     // No error, we did the operation
	return nil                            // Return no error, we did the operation
}
// ------------------------------------ //
// setval is a wrapper that passes the address of the value as the arg pointer
// ------------------------------------ //
func setval(id,num,v int) error {
  val:=v                                // Create a new int to hold the value
	_,_,e:=unix.Syscall6(unix.SYS_SEMCTL,uintptr(id),uintptr(num),
	  uintptr(SETVAL),uintptr(unsafe.Pointer(&val)),0,0) // Call the Sys_semctl syscall
	if e!=0{                              // Did we get an error?
	  return e                            // Yes, return the error.
	}                                     // No error, we set the value
	return nil                            // Return no error, we set the value
}                                       // -------------- setval ------------ //