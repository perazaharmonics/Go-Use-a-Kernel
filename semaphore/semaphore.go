//go:build linux
//+build linux

// ============================================================================
// Filename: Semaphore.go
// Description: System V semaphore implementation in Go, now currently needed  
// for concurrency control in the Proxy Server logging mechanism. Originally
// written in C++ by MJB, this is a Go port of the original code. 
//
// Author: 
//  JEP J. Enrique Peraza, Trivium Solutions LLC
//
// ============================================================================
package semaphore
import (
  "fmt"                                 // For string formatting.
	"os"                                  // For I/O, syscalls, etc.
  "os/user"                             // For the passwd struct.
	"syscall"
	"strconv"                             // For string conversion.
	"sync"                                // For mutexes.
	"time"                                // Timers, timeouts, etc.

	"golang.org/x/sys/unix"               // For semaphore syscalls.
)
const debug=false
const (
  semCount      = 3                     // 3 Semaphores per set.
	perm          = 0o700                 // Owner rwx permissions.
	initLock      = 1                     // Semaphore initial value for lock.
	initUserCount = 0                     // Inital value for user count.
)
// ------------------------------------ //
// Semaphore structure to wrap a System V semaphore set of size 3.
// where the first semaphore is the locking semaphore, the second is the
// user count, and the third is the locking semaphore for the user count.
// ------------------------------------ //
type Semaphore struct {
  key        int                        // The semaphore key.
	id         int                        // The semaphore set ID.
	who        string                     // The name of the caller.
	name       string                     // Record keeping for the caller.
	mu         sync.Mutex                 // Mutex for the semaphore.
	closed     bool                       // Flag to signal if semaphore is closed.
}                                       // Our semaphore structure.
// ------------------------------------ //
// Initializer for the semaphore creates or attaches to a semaphore set
// given a key. The semaphore set is created with 3 semaphores, the first
// is the locking semaphore, the second is the user count, and the third
// is the locking semaphore for the user count/
// ------------------------------------ //
func NewSemaphore(who,name,username string, key int) (*Semaphore, error){
  // Attempt to create a new semaphore set with the given key.
	// If the semaphore set already exists, attach to it.
	suid:=unix.Geteuid()                  // Get the effective user id.
	sgid:=unix.Getegid()                  // Get the effective group id.
	if username!=""{                      // Do we have a username?
	  u,err:=user.Lookup(username)        // Yes, lookup the user.
		if err!=nil{                        // Did we get the user?
		  return nil,fmt.Errorf("lookup user (%s): %s: %w",username,ErrSym(err),err)
		}                                   // Done checking if we got the user.
		uid, err := strconv.Atoi(u.Uid)     // Convert the UID from string to int.
		if err != nil{                      // Error converting UID?
		  return nil, fmt.Errorf("invalid uid %s: %w", u.Uid, err)
		}                                   // Done checking if we err converting UID.
		syscall.Seteuid(uid)                // Set the user ID...
		gid, err := strconv.Atoi(u.Gid)     // Convert the GID from string to int.
		if err != nil{                      // Error converting GID?
		  return nil, fmt.Errorf("invalid gid %s: %w", u.Gid, err)
		}                                   // Done checking if we err converting GID.
		syscall.Setegid(gid)                // Set the group ID...
	}                                     // Done checking if we have a username.
	id,err:=semget(key,semCount,unix.IPC_CREAT|unix.IPC_EXCL|perm)
	if err!=nil{                          // Could we create the semaphore?
	  if errno,ok:=err.(unix.Errno);ok&&errno==unix.EEXIST{// Does it already exist?
		  id,err=semget(key,semCount,perm)// Yes, so attach to it.
			if err!=nil{                      // Did we attach to it?
			  return nil,fmt.Errorf("attach sem (%s): %s: %w",name,ErrSym(err),err)
			}                                 // Done checking if error attaching.
		} else{                             // Else we could not create nor attach.
		  return nil,fmt.Errorf("create sem (%s): %s: %w",name,ErrSym(err),err)
		}                                   // Done checking if error creating.
	} else{                               // We created the semaphore.
	  if err:=initialize(id);err!=nil{    // Did we initialize the semaphore?
		  return nil,fmt.Errorf("init sem (%s): %s: %w",name,ErrSym(err),err)
		}                                   // Done checking if error initializing.
	}                                     // Done trying to initialize semaphore.
	s:=&Semaphore{key:key,id:id,who:who,name:name}// Create the semaphore object.
	if username!=""{                      // Did we change the user ID?
	  syscall.Seteuid(suid)               // Yes, restore our effective user ID.
		syscall.Setegid(sgid)               // Also,restore our effective group ID.
	}                                     // Done restoring user ID.
	if debug{                             // Are we debugging this?
	  s.logf("Opened semaphore %s with key %d and id %d.\n",name,key,id)
	}                                     // Done checking if debug.
	// ---------------------------------- //
	// We now created or attached to a semaphore set, so we need to
	// increment the user count semaphore to indicate that we are using
	// the semaphore set.
	// ---------------------------------- //
	if err:=s.IncrementUserCount();err!=nil{// Error incrementing user count?
	  return nil,fmt.Errorf("increment user count (%s): %s: %w",name,ErrSym(err),err)
	}                                     // Done incrementing user count.
	// ---------------------------------- //
	// If we got here, then we have a semaphore set that either we created
	// or attached to.
	// ---------------------------------- //
	return s,nil                          // Return the semaphore object, and no err.                     
}                                       // ---------- NewSemaphore ---------- //
// ------------------------------------ //
// Getters for the semaphore key
// ------------------------------------ //
func (s *Semaphore) GetKey() int{     // Get the semaphore key.
  return s.key                          // Return the semaphore key.
}                                       // ------------ GetKey -------------- //
func (s *Semaphore) GetID() int{        // Get the semaphore ID.
	return s.id                           // Return the semaphore ID.
}                                       // ------------ GetID --------------- //

// ------------------------------------ //
// Initialize the semaphore sets the locking semaphore to 1, the user count
// to 0, and the user count lock (not currently used) to 1.
// ------------------------------------ //
func initialize(id int) error{          // ----------- initialize ----------- //
  // For each semaphore in the set, and the values given for each index in the
	// slice...i=0,1,2 and v={0,1,0}
	for i,v:=range []int{initLock,initUserCount,initLock}{
	  if _,err:=semctl(id,i,SETVAL,v);err!=nil{// Did we set the value?
		  return err                        // No, so return the error.
		}                                   // Done initializing each semaphore.
	}                                     // Done initializing semaphore set.
	return nil                            // Return no error if we got here.
}                                       // ----------- initialize ----------- //
// ------------------------------------ //
// Lock acquires semaphore 0 (the locking semaphore) or blocks if it is already
// held by another process.
// ------------------------------------ //
func (s *Semaphore) Lock(why ...string) error{
  reason:=""                            // Default reason is empty.
	if len(why)>0{                        // Do we have a reason why?
	  reason=why[0]                       // Yes, assign it and use it.
	}                                     // Done checking if we have a reason why
	if debug{                             // Are we debugging this?
	  if v,err:=s.getVal(0);err!=nil{     // Did we get the value of sem[0]?
		  s.logf("Locking sem[%d]=%d (%s).\n",0,v,reason)
		}                                   // Done checking if got value of sem[0].
	}                                     // Done checking if debugging.
	if err:=s.semOp(0,-1);err!=nil{       // Did we get the lock?
    return fmt.Errorf("Lock failed (%s): %s: %w",reason,ErrSym(err),err)	
	}                                     // Done checking if we got the lock.
	if debug{                             // Are we debugging this?
	  if v,err:=s.getVal(0);err!=nil{     // Did we get the value of sem[0]?
		  s.logf("Locked sem[%d]=%d (%s).\n",0,v,reason)
		}                                   // Done checking if got value of sem[0].
	}                                     // Done checking if debugging.
	return nil                            // Return no error if we got here.
}                                       // -------------- Lock -------------- //
// ------------------------------------ //
// Unlock releases semaphore 0 (the locking semaphore).
// ------------------------------------ //
func (s *Semaphore) Unlock(why ...string) error{
  reason:=""                            // Default reason is empty.
	if len(why)>0{                        // Do we have a reason why?
	  reason=why[0]                       // Yes, so assign it and use it.
	}                                     // Done checking if we have a reason why.
	if debug{                             // Are we debugging this?
	  if v,err:=s.getVal(0);err!=nil{     // Did we get the value of sem[0]?
		  s.logf("Unlocking sem[%d]=%d (%s).\n",0,v,reason)
		}																	  // Done checking if got value of sem[0].
	}                                     // Done checking if debugging.
	if err:=s.semOp(0,+1);err!=nil{       // Did we release the lock?
	  return fmt.Errorf("Unlock failed (%s): %s: %w",reason,ErrSym(err),err)
	}                                     // Done checking if we released the lock.
	if debug{                             // Are we debugging this?
	  if v,err:=s.getVal(0);err!=nil{     // Did we get the value of sem[0]?
		  s.logf("Unlocked sem[%d]=%d (%s).\n",0,v,reason)
		}                                   // Done checking if got value of sem[0].
	}                                     // Done checking if debugging.
	return nil                            // Return no error if we got here.
}                                       // ------------ Unlock ------------- //
// ------------------------------------ //
// IncrementUserCount inrements the user count semaphore (sem[1]) by 1.
// ------------------------------------ //
func (s *Semaphore) IncrementUserCount() error{
  return s.modifyUserCount(+1)          // Increment the user count semaphore.
}                                       // --------- IncrementUserCount ----- //
// ------------------------------------ //
// DecrementUserCount decrements the user count semaphore (sem[1]) by 1.
// (protected by the user count lock sem[2])
// ------------------------------------ //
func (s *Semaphore) DecrementUserCount() error{
  return s.modifyUserCount(-1)          // Decrement the user count semaphore.
}                                       // --------- DecrementUserCount ----- //
// ------------------------------------ //
// modifyUserCount modifies the user count semaphore (sem[1]) by the given
// value. The value can be positive or negative.
// ------------------------------------ //
func (s *Semaphore) modifyUserCount(delta int) error{
  if debug{                             // Are we debugging this?
	  s.logf("Modifying user count by %d.\n",delta)
	}                                     // Done checking if debugging.
	// ---------------------------------- //
	// Lock the user count lock semaphore (sem[2])
	// to protect the user count semaphore (sem[1]).
	// ---------------------------------- //
	if err:=s.semOp(2,-1);err!=nil{       // Did we get the lock?
	  return fmt.Errorf("Lock user count: %s: %w",ErrSym(err),err)
	}                                     // Done checking if we got the lock.
	// ---------------------------------- //
	// Read the current value of the user count semaphore (sem[1]).
	// ---------------------------------- //
	curr,err:=s.getVal(1)                    // Get the current value of sem[1].
	if err!=nil{                          // Did we get the value of sem[1]?
	  _=s.semOp(2,+1)                     // Release the user count lock.
		s.logf("Error getting user count: %s.\n",ErrSym(err))
	}                                     // Done checking if got value of sem[1].
	// ---------------------------------- //
	// Set the new value of the user count semaphore (sem[1]).
	// ---------------------------------- //
  newv:=curr+delta                      // Set the new value of sem[1].
	if _,err:=semctl(s.id,1,SETVAL,newv);err!=nil{
	  _=s.semOp(2,+1)                     // Release the user count lock.
		return fmt.Errorf("set user count: %s: %w",ErrSym(err),err)
	}                                     // Done setting the new value of sem[1].
	// ---------------------------------- //
	// Release the user count lock semaphore (sem[2]).
	// ---------------------------------- //
	if err:=s.semOp(2,+1);err!=nil{       // Did we release the lock?
	  return fmt.Errorf("Unlock user count: %s: %w",ErrSym(err),err)
	}                                     // Done checking if we released the lock.
	if debug{                             // Are we debugging this?
	  s.logf("User count changed from %d to %d.\n",curr,newv)
	}                                     // Done checking if debugging.
	return nil                            // Return no error if we got here.
}                                       // --------- modifyUserCount --------- //
// ------------------------------------ //
// GetUserCount return the current value of the user count semaphore (sem[1]).
// ------------------------------------ //
func (s *Semaphore) GetUserCount() (int,error){
  return s.getVal(1)                    // Return the value of sem[1].
}                                       // ----------- GetUserCount ----------- //
// ------------------------------------ //
// IsLocked verifies whether semaphore 0 (the locking semaphore) is locked.
// ------------------------------------ //
func (s *Semaphore) IsLocked() (bool,error) {
  v,err:=s.getVal(0)                    // Get the value of sem[0].
	return v==0,err                       // Return true if sem[0] is locked.
}                                       // ----------- IsLocked ------------ //
// ------------------------------------ //
// ClearUserCount clears the user count of semaphore (sem[1]) to 0.
// ------------------------------------ //
func (s *Semaphore) ClearUserCount() error{
	if _,err:=semctl(s.id,1,SETVAL,0);err!=nil{// Can we clear it?
	  return fmt.Errorf("clear user count: %s: %w",ErrSym(err),err)
	}                                     // Done checking if we can clear it.
  return nil                            // Return no error if we got here.
}                                       // --------- ClearUserCount --------- //
// ------------------------------------ //
// Remove deletes the semaphore set immediately.
// ------------------------------------ //
func (s *Semaphore) Remove() error{
  if debug{                             // Are we debugging this?
	  s.logf("Removing semaphore %s with key %d and id %d.\n",s.name,s.key,s.id)
	}                                     // Done checking if debugging.
	if _,err:=semctl(s.id,0,unix.IPC_RMID,0);err!=nil{// Can we remove it?
	  return fmt.Errorf("Remove sem (%s): %s: %w",s.name,ErrSym(err),err)
	}                                     // Done checking if we can remove it.
	return nil														// Return no error if we got here.
}                                       // ------------- Remove ------------- //
// ------------------------------------ //
// Close decrements the user count and removes the semaphore set if the
// user count is 0.
// ------------------------------------ //
func (s *Semaphore) Close() error{
  s.mu.Lock()                           // Lock the mutex to protect the sem structure.
	defer s.mu.Unlock()                   // Unlock the mutex when we are done.
	if s.closed{                          // Is the sem set already closed?
	  return nil                          // Yes so return no error.
	}                                     // Done checking if already closed.
  if err:=s.DecrementUserCount();err!=nil{// Did we decrement the user count?
	  return fmt.Errorf("decrement user count: %s: %s: %w", s.name, ErrSym(err), err)
	}                                     // Done checking if decremented user count.
	// ---------------------------------- //
	// If the user count is 0, then we can remove the semaphore set.
	// ---------------------------------- //
	if uc,_:=s.GetUserCount();uc==0{      // Is the user count 0?
	  if debug{                           // Are we debugging this?
		  s.logf("User count is 0, removing semaphore %s with key %d and id %d.\n",s.name,s.key,s.id)
		}                                   // Done checking if in debug mode.
		if _,err:=semctl(s.id,0,unix.IPC_RMID,0);err!=nil{// Can we remove it?
		  return fmt.Errorf("Remove sem (%s): %s: %w",s.name,ErrSym(err),err)
		}																	  // Done checking if we can remove it.
	}                                     // Done checking if user count is 0.
	s.closed=true                         // Set the closed flag to true.
	return nil                            // Return no error if we got here.
}                                       // -------------- Close ------------- //
// ------------------------------------ //
// semOp performs a semaphore operation on the given semaphore operation
// on the given semaphore index (i.e. sem[i]) and the given operation given by
// the op value. Lock is -1, unlock is +1.
// ------------------------------------ //
func (s *Semaphore) semOp(i int,op int16) error{
// Fields are index, value to add, and flags.
// The flags are 0 for no flags, and the value to add is the operation.
// The index is the semaphore index (0,1,2).
// The value to add is the operation (lock or unlock).
  sb:=[]sembuf{{SemNum: uint16(i), SemOp: op, SemFlg: 0}}
	for {                                 // Loop until we get the semaphore op.
	  if err:=semop(s.id,sb);err!=nil{// Did we get the semaphore operation?
		  if errno,ok:=err.(unix.Errno);ok&&errno==unix.EINTR{// Interrupted by a signal?
			  time.Sleep(10*time.Microsecond) // Yes so wait a little before retrying.
				continue                        // Retry the semaphore operation.
			}                                 // Done checking if interrupted by signal.
			return err                        // could not get sem operation return err.
		}                                   // Done checking if we got sem operation.
		// -------------------------------- //
		// If we got here, then we were not interrupted by any other signal that is
		// not SIGINT or SIGTERM, so we can return nil error because we got the
		// semaphore operation done successfully.
		// -------------------------------- //
		return nil                          // Return no error if we got here.                        
	}                                     // Done trying to do semop.
}                                       // ------------- semOp -------------- //
// ------------------------------------ //
// getVal is a wrapper for the semctl(s.id, semaphore index, GETVAL)
// system call.
// ------------------------------------ //
func (s *Semaphore) getVal(i int) (int, error){
  v,err:=semctl(s.id,i,GETVAL,0)// Get the value of this semaphore.
	return v,err                          // Return the value of sem[i] and error.
}                                       // ------------- getVal ------------- //
// ------------------------------------ //
// logf is just a wrapper for the fmt.Fprintf function to log to stderr
// with a nice header.
// ------------------------------------ //
func (s *Semaphore) logf(format string, args ...interface{}){
  t:=time.Now()                         // Get the current time.
	hdr:=fmt.Sprintf("%12d.%09d pid=%d %s::%s ",t.Unix(),t.Nanosecond(),
	  os.Getpid(),s.who,s.name)
	fmt.Fprint(os.Stderr,hdr)
	fmt.Fprintf(os.Stderr,format,args...) // Print the formatted string.
}                                       // ------------- logf -------------- //
// ------------------------------------ //
// ErrSym is a wrapper for the unix.Errno type to get the string
// representation of the error.
// ------------------------------------ //
func ErrSym(err error) string{
  if errno,ok:=err.(unix.Errno);ok{
	  switch errno{                       // Return the symbol for these errors.
		  case unix.EACCES: return "EACCES"
			case unix.EEXIST:  return "EEXIST"
			case unix.EINVAL:  return "EINVAL"
			case unix.ENOENT:  return "ENOENT"
			case unix.ENOMEM:  return "ENOMEM"
			case unix.ENOSPC:  return "ENOSPC"
			case unix.E2BIG:   return "E2BIG"
			case unix.EAGAIN:  return "EAGAIN"
			case unix.EFAULT:  return "EFAULT"
			case unix.EFBIG:   return "EFBIG"
			case unix.EIDRM:   return "EIDRM"
			case unix.EINTR:   return "EINTR"
			case unix.ERANGE:  return "ERANGE"
		}                                   // Done checking for known errors.
		return errno.Error()                // Return the error string of errno if none.
	}                                     // Done checking if error is unix.Errno.
	return err.Error()                    // Return go err string if not unix.Errno.
}                                       // ------------- ErrSym ------------- //

func (s *Semaphore) ForceUnlock() error{
  return initialize(s.id)              // Force unlock the semaphore.
}                                      // ----------- ForceUnlock ----------- //