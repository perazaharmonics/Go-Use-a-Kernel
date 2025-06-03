// **************************************************************************
// Filename:
//
//	configuration.go
//
// Description:
//
//	Implementation of the configuration package.
//
// Author:
//
//	MJB  Matthew J. Bieneman
//
// Translated by:
//
//	J.EP  J. Enrique Peraza
//
//
// ** *************************** // Notes // **************************** ** //
// **  1. The normal way to use this class is to create a customized       ** //
// **      configuration object for your application which uses            ** //
// **      Configuration as its base class. That object should be created  ** //
// **      by the constructor of whatever class you have that has          ** //
// **      Application as its base class.                                  ** //
// **  2. Besides the configuration for a whole Application class, this    ** //
// **      class is used to handle all kinds of other data stored in the   ** //
// **      "configuration file" format. (Sections in square brackets;      ** //
// **      containing named Parameters with '=' signs and/or unnamed       ** //
// **      Parameters without '=' signs.                                   ** //
// ************************************************************************** //
package configuration

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)
const debug=true
const uselog=true
var(
  errs       []error
	comments   *Comment
	Config     *Configuration
)
// =========================== // Helpers // ==================================
func isTrue(p string) bool{
  p = strings.ToLower(strings.TrimSpace(p))
	return p == "true"
}
func isFalse(p string) bool{
  p = strings.ToLower(strings.TrimSpace(p))
	return p == "false"
}
func isPointer(v any) bool { return reflect.ValueOf(v).Kind() == reflect.Ptr }
func verbComaptible(verb byte,k reflect.Kind) bool{
  switch verb{
	  case 'd','x','o','b','c','U':       // Integer types and pointers.
		  return k>=reflect.Int&&k<=reflect.Uintptr  
		case 'f','e','E','g','G':           // Floating point types
		  return k==reflect.Float32||k==reflect.Float64 
		case 't':                           // Boolean type.
		  return k==reflect.Bool
		case 's','q','v':                   // String and composite types.
		  return k==reflect.String||k==reflect.Struct||
		    k==reflect.Array||k==reflect.Slice||
		    k==reflect.Map||k==reflect.Ptr||
		    k==reflect.Interface||k==reflect.Chan||
		    k==reflect.Func||k==reflect.UnsafePointer 
		default:                            // Let fmt.Sscanf() handle it.
		  return true            
	}
}

// =========================== // Comment // ==================================
// A class to store comments and blank lines from a configuration file.
// ============================================================================
// The Comment constructor.
func NewComment(value string, imported bool) *Comment{
  if value != "" {                        // Anything to store?
		return &Comment{
			imports: strings.HasPrefix(strings.TrimSpace(value), "import"),
			isimported: imported,
			value: strings.TrimSpace(value),
		}// Done creating the Comment object.
	}                                     // Done checking for our purpose.
  return nil                            // Nothing to store, return nil.
}                                       // ------------ NewComment ---------- //
// Comment copy constructor.
func CopyComment(c *Comment) *Comment{
  if c!=nil{                            // Anything to copy?
	  return &Comment{                    // Yes, return a copy of Comment object.
		  imports: c.imports,               // Copy the imports.
			isimported: c.isimported,         // Copy the imported flag.
			value: strings.TrimSpace(c.value),  // Copy the value.
		}                                   // Done copying the Comment object.
	}                                     // Done checking for our purpose.
	return nil                            // Nothing to copy, return nil.
}                                       // ------------ CopyComment --------- //
// ------------------------------------ //
// Inline getters and setters for the Comment object.
// ------------------------------------ //
func (c *Comment) GetValue() string{ return c.value }                                     
func (c *Comment) GetNext() *Comment{ return c.next }                                      
func (c *Comment) SetNext(p *Comment){ if c!=nil{ c.next=p } }                                       
func (c *Comment) IsImported() bool{ return c.isimported}                                      
func (c *Comment) IsImportStatement() bool{
	if c!=nil{                            // Anything to get?
	  return c.imports                    // Yes, return the imports flag.
	}                                     // Done getting imports flag.
	return false                          // Nothing to get, return false.
}                                       // ------- IsImportStatement -------- //
// ---------------------------- // Print // --------------------------------- //
// Print the object to the stream output.
// -------------------------------------------------------------------------- //
func (c *Comment) Print(w io.Writer) error{
  if c!=nil{                            // Anything to print?
	  _,err:=fmt.Fprintln(w,string(c.value)) // Yes, print the value.
		return err                          // Done printing the Comment object.
	}                                     // Done checking for our purpose.
	return nil                            // Nothing to print, return nil.
}
// ========================= // Parameter // ==================================
// A class to store individual parameters and their values.
// ============================================================================
// ------------------------- // constructor // ------------------------------ //
// Initialize the parameter data structures. Note that comments are always
// allocated by the calling method. This allows the caller to build up a list
// of comments (which it finds before it finds the parameter name) and then just
// pass a pointer to that list to this constructor.
// -------------------------------------------------------------------------- //

func NewParameter(name, valuestr string,comments *Comment, imported bool) *Parameter{
  p:=&Parameter{
	  name: strings.TrimSpace(name),      // Copy the name.
		comments: comments,                 // Copy the comments.
		isimported: imported,               // Copy the imported flag.
	}                                     // Done creating the Parameter object.
	_=p.SetValue(valuestr,0)              // Set the value(s) for this Parameter.
	return p                              // Return the new Parameter object.
}                                       // -------- NewParameter ------- //
// ------------------------- // Copy constructor // ------------------------- //
// Initialize the parameter data structures. The copy will not be "imported". //
// -------------------------------------------------------------------------- //

func CopyParameter(p *Parameter) *Parameter{
  if p.comments!=nil{                    // Any comments to copy?
    comments=CopyComment(p.comments)     // Yes, copy the comments.
  }
  n:=p.n                                // Copy the number of values.
  name := p.name                        // Copy the name
  var value string                      // Initialize an empty string
  if p.value != "" {                    // Any value to copy?
	  value = p.value                     // Copy the source string.
  }                                     // Done copying the values.
 var values []string=nil                // Where to copy the values.
 var quotes []byte=nil                  // Where to copy the quotes.
 if n!=0{                               // Are there any values?
	 values= make([]string, p.n)          // Yes, allocate space for the values.
	 quotes = make([]byte, p.n)           // Yes, allocate space for the quotes.
  for i := 0; i < int(n); i++ {              // For each value...
	  values[i] = p.values[i]             // Copy the value.
		quotes[i] = p.quotes[i]             // Copy the quote.
	}                                     // Done copying the values.
 }                                      // Done checking if values exits.
	return &Parameter{                    // Return the new Parameter object.
	  name: strings.TrimSpace(name),      // Copy the name.   
		n:    n,                            // Copy the number of values.
		comments: comments,                 // Copy the comments.
		isimported: false,                  // Copy the imported flag.
		value: value,                       // Copy the value.
		values: values,                     // Copy the values.
		quotes: quotes,                     // Copy the quotes.
	}                                     // Done copying the Parameter object.
}                                       // -------- CopyParameter -------- //
// ------------------------------------ //
// Parameter getters and setters.
// ------------------------------------ //
func (p *Parameter) IsImported() bool{ return p.isimported }
func (p *Parameter) GetNValues() uint{ return p.n }
func (p *Parameter) GetValueArray() []string{ return p.values }
func (p *Parameter) GetValue(i uint) string{
  if i<uint(p.n){ return p.values[i]} else{ return "" } 
}
func (p *Parameter) GetValues() string { return p.value}
func (p *Parameter) GetName() string{ return p.name }
func (p *Parameter) GetNext() *Parameter{ return p.next }
func (p *Parameter) SetNext(p2 *Parameter){ if p!=nil{ p.next=p2 } }

// ------------------------ // GetValueByte() // ---------------------------- //
// Get the value of a parameter as a byte (8-bit character)
// -------------------------------------------------------------------------- //
func (p *Parameter) GetValueByte(value string, dest *byte) error{
  if len(value)==0{                     // Where we given a value to decode? 
	  return fmt.Errorf("can't decode empty \"value\" to byte")// No, that's bad.
	}                                     // Done checking for empty value.
	return p.scanValue("%c",dest)        // Decode the value into the destination byte.
}                                       // --------- GetValueByte ----------- //
// ------------------------ // GetValueOfIndexByte() // --------------------- //
// Get the value of a multi-valued Parameter as a byte (8-bit character).
// -------------------------------------------------------------------------- //
func (p *Parameter) GetValueByteByIndex(i uint,dest *byte) error{
  if int(i)>=len(p.values){              // Where we given a value to decode?
	  return fmt.Errorf("can't decode empty \"value\" to byte")// No, that's bad.
	}                                     // Done checking for empty value.
	return p.scanValueByIndex(int(i),"%c",dest)//Decode value into destination byte.
}                                       // ------- GetValueByteByIndex ------ //

// ------------------------ // GetValueTime() // ---------------------------- //
// Get the value of a parameter as a time.Time object. The value must be in a
// format that can be parsed by time.Parse(). The format is:
//   "2006-01-02 15:04:05" or "2006-01-02T15:04:05Z07:00" or "2006-01-02T15:04:05Z"
// -------------------------------------------------------------------------- //
func (p *Parameter) GetValueTime(value string, dest *time.Time) error{
  if len(value)==0{                     // Where we given a value to decode?
	  return fmt.Errorf("can't decode empty \"value\" to time.Time")// No, that's bad.
	}                                     // Done checking for empty value.
	t,err:=time.Parse(time.RFC3339,value) // Parse the value as a time.
	if err!=nil{                          // Any error parsing the time?
	  return fmt.Errorf("can't decode \"%s\" to time.Time: %v", value, err)
	}                                     // Done checking for parse error.
	*dest=t                               // Set the destination time to the parsed time.
	return nil                            // Return nil if we got here.   
}                                       // -------- GetValueTime ------------ //
func (p *Parameter) GetValueTimeByIndex(i uint, dest *time.Time) error{
  if i>=uint(len(p.values)){            // Is the index out of range?
	  return fmt.Errorf("index %d out of range", i)// Yes, panic.
	}                                     // Done checking for out of range index.
	q:=p.values[i]                        // Get the value at the index.
	t,err:=time.Parse(time.RFC3339,q)     // Parse the value as a time.
	if err!=nil{                          // Any error parsing the time?
	  return fmt.Errorf("can't decode \"%s\" to time.Time: %v", q, err)
	}                                     // Done checking for parse error.
	*dest=t                               // Set the destination time to the parsed time.
	return nil                            // Return nil if we got here.   
}                                       // ---- GetValueTimeByIndex -------- //
func (p *Parameter) GetValueTimespec(value string, dest *unix.Timespec) error{
  if len(value)==0{                     // Where we given a value to decode?
	  return fmt.Errorf("can't decode empty \"value\" to unix.Timespec")// No, that's bad.
	}                                     // Done checking for empty value.
	t,err:=time.Parse(time.RFC3339,value) // Parse the value as a time.
	if err!=nil{                          // Any error parsing the time?
	  return fmt.Errorf("can't decode \"%s\" to unix.Timespec: %v", value, err)
	}                                     // Done checking for parse error.
	unix.NsecToTimespec(t.UnixNano()) 	  // Convert the time to a unix.Timespec.
	*dest=unix.NsecToTimespec(t.UnixNano()) // Set the destination time to the parsed time.
	return nil                            // Return nil if we got here.   
}                                       // -------- GetValueTimespec -------- //
func (p *Parameter) GetValueTimespecByIndex(i uint,dest *unix.Timespec)error{
  if i>=uint(len(p.values)){            // Is the index out of range?
	  return fmt.Errorf("index %d out of range", i)// Yes, panic.
	}                                     // Done checking for out of range index.
	q:=p.values[i]                        // Get the value at the index.
	t,err:=time.Parse(time.RFC3339,q)     // Parse the value as a time.
	if err!=nil{                          // Any error parsing the time?
	  return fmt.Errorf("can't decode \"%s\" to time.Time: %v", q, err)
	}                                     // Done checking for parse error.
	unix.NsecToTimespec(t.UnixNano()) 	  // Convert the time to a unix.Timespec.
	*dest=unix.NsecToTimespec(t.UnixNano()) // Set the destination time to the parsed time.
	return nil                            // Return nil if we got here.
}                                       // ---- GetValueTimeByIndex --------- //
func (p *Parameter) GetValueDuration(value string, dest *time.Duration) error{
  if len(value)==0{                     // Where we given a value to decode?
	  return fmt.Errorf("can't decode empty \"value\" to time.Duration")// No, that's bad.
	}                                     // Done checking for empty value.
	d,err:=time.ParseDuration(value)      // Parse the value as a duration.
	if err!=nil{                          // Any error parsing the duration?
	  return fmt.Errorf("can't decode \"%s\" to time.Duration: %v", value, err)
	}                                     // Done checking for parse error.
	*dest=d                               // Set the destination duration to the parsed duration.
	return nil                            // Return nil if we got here.   
}                                       // -------- GetValueDuration -------- //
func (p *Parameter) GetValueDurationByIndex(i uint, value string,dest *time.Duration) error{
  if i>=uint(len(p.values)){            // Is the index out of range?
	  return fmt.Errorf("index %d out of range", i)// Yes, panic.
	}                                     // Done checking for out of range index.
	q:=p.values[i]                        // Get the value at the index.
	d,err:=time.ParseDuration(q)          // Parse the value as a duration.
	if err!=nil{                          // Any error parsing the duration?
	  return fmt.Errorf("can't decode \"%s\" to time.Duration: %v", q, err)
	}                                     // Done checking for parse error.
	*dest=d                               // Set the destination duration to the parsed duration.
	return nil                            // Return nil if we got here.   
}                                       // ---- GetValueDurationByIndex ----- //

// ----------------------- GetValueInt -------------------------------------- //
// Decode a value of a parameter into the given format, and place it into the
// given destination integer.
// -------------------------------------------------------------------------- //
func (p *Parameter) GetValueInt(value string, dest *int) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to int")
	}                                     
	return p.scanValue("%d",dest)        
}                                       
func (p *Parameter)	GetValueIntByIndex(i uint,dest *int) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to byte")
	}                                     
	return p.scanValueByIndex(int(i),"%d",dest)
}                                       
func (p *Parameter) GetValueInt8(value string, dest *int8) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to int8")
	}                                     
	return p.scanValue("%d",dest)        
}                                       
func (p *Parameter)	GetValueInt8ByIndex(i uint,dest *int8) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to int8")
	}                                     
	return p.scanValueByIndex(int(i),"%d",dest)
}                                       
func (p *Parameter) GetValueInt16(value string, dest *int16) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to int16")
	}                                     
	return p.scanValue("%d",dest)        
}                                       
func (p *Parameter)	GetValueInt16ByIndex(i uint,dest *int16) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to int16")
	}                                     
	return p.scanValueByIndex(int(i),"%d",dest)
}                                       
func (p *Parameter) GetValueInt32(value string, dest *int32) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to int32")
	}                                     
	return p.scanValue("%d",dest)        
}                                       
func (p *Parameter)	GetValueIn32tByIndex(i uint,dest *int32) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to int32")
	}                                     
	return p.scanValueByIndex(int(i),"%d",dest)
}                                       
func (p *Parameter) GetValueInt64(value string, dest *int64) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to int64")
	}                                     
	return p.scanValue("%d",dest)        
}                                       
func (p *Parameter)	GetValueInt64ByIndex(i uint,dest *int64) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to int64")
	}                                     
	return p.scanValueByIndex(int(i),"%d",dest)
}                                       
func (p *Parameter)	GetValueRune(value string, dest *rune) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to rune")
	}
	return p.scanValue("%c",dest)
}
func (p *Parameter)	GetValueRuneByIndex(i uint,dest *rune) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to rune")
	}
	return p.scanValueByIndex(int(i),"%c",dest)
}
func (p *Parameter)	GetValueBinary(value string, dest *string) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to binary")
	}                                     
	return p.scanValue("%b",dest)        
}
func (p *Parameter)	GetValueHex(value string, dest *string) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to hex")
	}                                     
	return p.scanValue("%x",dest)        
}
func (p *Parameter)	GetValueOctal(value string, dest *string) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to octal")
	}                                     
	return p.scanValue("%o",dest)        
}
// ----------------------- Unsigned Integers -------------------------------- //

func (p *Parameter) GetValueUint(value string, dest *uint) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to uint")
	}                                     
	return p.scanValue("%u",dest)        
}                                       
func (p *Parameter)	GetValueUintByIndex(i uint,dest *uint) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to uint")
	}                                     
	return p.scanValueByIndex(int(i),"%u",dest)
}                                       
func (p *Parameter) GetValueUint8(value string, dest *uint8) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to uint")
	}                                     
	return p.scanValue("%u",dest)        
}                                       
func (p *Parameter)	GetValueUint8ByIndex(i uint,dest *uint8) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to uint")
	}                                     
	return p.scanValueByIndex(int(i),"%u",dest)
}   
func (p *Parameter) GetValueUint16(value string, dest *uint16) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to uint16")
	}                                     
	return p.scanValue("%u",dest)        
}                                       
func (p *Parameter)	GetValueUint16ByIndex(i uint,dest *uint16) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to uint16")
	}                                     
	return p.scanValueByIndex(int(i),"%u",dest)
}
func (p *Parameter) GetValueUint32(value string, dest *uint32) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to uint32")
	}                                     
	return p.scanValue("%u",dest)        
}                                       
func (p *Parameter)	GetValueUint32ByIndex(i uint,dest *uint32) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to uint32")
	}                                     
	return p.scanValueByIndex(int(i),"%u",dest)
}
func (p *Parameter) GetValueUint64(value string, dest *uint64) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to uint32")
	}                                     
	return p.scanValue("%u",dest)        
}                                       
func (p *Parameter)	GetValueUint64ByIndex(i uint,dest *uint64) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to uint64")
	}                                     
	return p.scanValueByIndex(int(i),"%u",dest)
}                         
// ---------------------- Floating point values ----------------------------- //
func (p *Parameter)	GetValueFloat32(value string, dest *float32) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to float32")
	}                                     
	return p.scanValue("%f",dest)        
}
func (p *Parameter)	GetValueFloat32ByIndex(i uint,dest *float32) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to float32")
	}                                     
	return p.scanValueByIndex(int(i),"%f",dest)
}
func (p *Parameter)	GetValueFloat64(value string,dest *float64) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to float64")
	}                                     
	return p.scanValue("%f",dest)        
}
func (p *Parameter)	GetValueFloat64ByIndex(i uint,dest *float64) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to float64")
	}                                     
	return p.scanValueByIndex(int(i),"%f",dest)
}
	// Scientific notation
func (p *Parameter)	GetValueSI(value string,dest *string) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to scientific notation")
	}                                     
	return p.scanValue("%e",dest)        
}
// ------------------- Floating point values with precision ----------------- //
func (p *Parameter)	GetValuePrecisionFloat32(value,precision string,dest *float32) error{
  if len(value)==0{              
	  return fmt.Errorf("can't decode empty \"value\" to float32")
	}
	return p.scanValue(fmt.Sprintf("%%.%sf", precision), dest)  
}
func (p *Parameter)	GetValuePrecisionFloat32ByIndex(i uint,precision string,dest *float32) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to float32")
	}
	return p.scanValueByIndex(int(i), fmt.Sprintf("%%.%sf", precision), dest)
}
func (p *Parameter)	GetValuePrecisionFloat64(value,precision string,dest *float64) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to float64")
	}                                     
	return p.scanValue(fmt.Sprintf("%%.%sf", precision), dest)  
}
func (p *Parameter)	GetValuePrecisionFloat64ByIndex(i uint,precision string,dest *float64) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to float64")
	}                                     
	return p.scanValueByIndex(int(i), fmt.Sprintf("%%.%sf", precision), dest)
}

// ---------------------- Complex numbers ----------------------------------- //
func (p *Parameter)	GetValueComplex64(value string,dest *complex64) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to complex64")
	}                                     
	return p.scanValue("%v",dest)        
}  

func (p *Parameter)	GetValueComplex64ByIndex(i uint,dest *complex64) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to complex4")
	}                                     
	return p.scanValueByIndex(int(i),"%v",dest)
}
func (p *Parameter)	GetValueComplex128(value string,dest *complex128) error{
  if len(value)==0{                     
	  return fmt.Errorf("can't decode empty \"value\" to complex128")
	}                                     
	return p.scanValue("%v",dest)        
}
func (p *Parameter)	GetValueComplex128ByIndex(i uint,dest *complex128) error{
  if int(i)>=len(p.values){              
	  return fmt.Errorf("can't decode empty \"value\" to complex128")
	}                                     
	return p.scanValueByIndex(int(i),"%v",dest)
}

// ----------------------- ScanValueOfIndex --------------------------------- //
// Decode a value of a multi-valued Parameter into the given format, and place
// if into the given buffer so it can be obtained by address.
// The format is one of the following:
//   "%d"       - signed integer
//   "%u"       - unsigned integer
//   "%b"       - Base 2 (binary) representation of the value.
//   "%o"       - unsigned octal integer
//   "%x"       - unsigned hex integer     
//   "%e"       - float in scientific notation
//   "%g"       - float in decimal notation
//   "%f"       - float
//   "%lf"      - double
//   "%s"       - string
//   "%t"       - boolean
//   "%c"       - character
//   "%U"       - unicode character
//   "%q"       - quoted character
//   "%p"       - pointer
//   "%p"       - Base 16 notation of pointer address.
//   "%v"       - Composite type (structs, arrays, slices, maps, and so on...)
//   "%<width>d"-  signed integer with width
//   "%0<width>d"- signed integer with width and leading zeros
//   "%<width>u"-  unsigned integer with width
//   "%0<width>u"-  unsigned integer with width and leading zeros
//   "%.<precision>f"-  float with precision
//   "%<width>.<precision>f"- float with width and precision
//   "%<width>s" - Pad string with spaces set to width.
// -------------------------------------------------------------------------- //
func (p *Parameter) scanValue(format string, dest any) error{
  if len(p.value)==0{       // Within range?
	return fmt.Errorf("The value of the parameter %s is %v",p.name,p.value)
  }                                     // Done checking for out of range.
  var verb byte                         // The format verb.
  for j:=1;j<len(format);j++{           // For each character in fmt string...
	  c:=format[j]                        // Get the j'th characer.
		// -------------------------------- //
		// We need to check if 'c' is one of the characters that can legally
		// appear in the *flags, width, precision or modifier* part of a fmt string.
		// if it is, we will skip it and continue scanning the next character.
		// -------------------------------- //
		if strings.ContainsRune("#0- +. '0123456789*",rune(c)){ continue }
		// -------------------------------- //
		// Otherwise we have reached the first character that is not a flag, digit,
		// dot or star, so by definition this is the format verb.
		// -------------------------------- //
		verb=c                              // The format verb.
		break                               // We found the verb so break from loop.
	}                                     // Done iterating through the format string.
  if verb!=0&&!verbComaptible(verb,reflect.TypeOf(dest).Elem().Kind()){
	  return fmt.Errorf("format %s is not compatible with type %s", format, reflect.TypeOf(dest).Elem().Kind())
	}                                     // Done checking for format compatibility.
  val:=reflect.ValueOf(dest).Kind()     // Get the type of the dest variable.
	if dest==nil||val!=reflect.Ptr{       // Is dest nil or not a pointer?
	return errors.New("destination must be a non-nil pointer")// Yes, return an error.
  }                                     // Done checking for nil or pointer.
  //raw:=p.GetValue(uint(i))                              
	// Scan the value into the destination variable
  _,err:=fmt.Sscanf(string(p.value),format,dest)
	return err                            // Return error if any.  
}
// ----------------------- scanValueByIndex --------------------------------- //
func (p *Parameter) scanValueByIndex(i int, format string, dest any) error{
  if i < 0 || i >= len(p.values){       // Within range?
	return fmt.Errorf("index %d out of range", i)
  }                                     // Done checking for out of range.
  var verb byte                         // The format verb.
  for j:=1;j<len(format);j++{           // For each character in fmt string...
	  c:=format[j]                        // Get the j'th characer.
		// -------------------------------- //
		// We need to check if 'c' is one of the characters that can legally
		// appear in the *flags, width, precision or modifier* part of a fmt string.
		// if it is, we will skip it and continue scanning the next character.
		// -------------------------------- //
		if strings.ContainsRune("#0- +. '0123456789*",rune(c)){ continue }
		// -------------------------------- //
		// Otherwise we have reached the first character that is not a flag, digit,
		// dot or star, so by definition this is the format verb.
		// -------------------------------- //
		verb=c                              // The format verb.
		break                               // We found the verb so break from loop.
	}                                     // Done iterating through the format string.
  if verb!=0&&!verbComaptible(verb,reflect.TypeOf(dest).Elem().Kind()){
	  return fmt.Errorf("format %s is not compatible with type %s", format, reflect.TypeOf(dest).Elem().Kind())
	}                                     // Done checking for format compatibility.
  val:=reflect.ValueOf(dest).Kind()     // Get the type of the dest variable.
	if dest==nil||val!=reflect.Ptr{       // Is dest nil or not a pointer?
	return errors.New("destination must be a non-nil pointer")// Yes, return an error.
  }                                     // Done checking for nil or pointer.
  //raw:=p.GetValue(uint(i))                              
	// Scan the value into the destination variable
  _,err:=fmt.Sscanf(string(p.values[i]),format,dest)
	return err                            // Return error if any.
}                                       // --------- ScanValueOfIndex -------- //
// ----------------------------- // GetValueBool // ------------------------- //
//  Get the value of a bool parameter. If values are given for true and       //
// false, then the result must match one or the other. If you give either a   //
// true or false value, but not the other, then the result will always be     //
// false. If tval and fval are the same, then the result will always be true. //
//  If the value does not match either of the tval and fval values, the       //
// result will not be set and the method will return false.                   //
// -------------------------------------------------------------------------- //
func (p *Parameter) GetValueBool(i uint,tval string, fval string) (result bool, err error){
  if tval==""&&fval==""{              // Did calles give possible values?
	  if isTrue(p.values[i]){             // No. Is it "true"?
		  result=true                       // Yes, set result to true.
		} else if isFalse(p.values[i]){     // Is it "false""?"
		  result=false                      // Yes, set result to false.
		} else{                             // Else return error.
		  result=false
			err=fmt.Errorf("value %s is not a boolean", p.values[i])
		}                                   // Done checking for boolean.
	} else if tval!=""&&fval!=""{       // Caller gave possible values?            
	  if strings.Contains(tval,p.values[i]){   // Yes, is it the true value?
		  result=true                       // Yes, set result to true.
		} else if strings.Contains(fval,p.values[i]){ // Is it the false value?
		  result=false                      // Yes, set result to false.
		} else{                             // Else we have an error.
		  result=false                      // Set result to false.
			err=fmt.Errorf("value %s is not a boolean", p.values[i])// Store error.
		}                                   // Done checking for boolean.
	} else{                               // Else only one of the values is given.
	  result=false                        // Set result to false.
		err=fmt.Errorf("only one of the values was given.")// Store error.
	}                                     // Done checking for one value.
	return result, err                    // Return the result and error.
}                                       // ----------- GetValueBool --------- //
// --------------------------- // Append // --------------------------------- //
// Place another parameter in the list after this one. This must only be called
// for the last parameter in the list.
// -------------------------------------------------------------------------- //
func (p *Parameter) Append(p2 *Parameter){
  p.next=p2                             // Set the next parameter to p2.
}                                       // ------------ Append ------------- //
// ----------------------------- // SetValue // ----------------------------- //
//  Set the value(s) for this Parameter. Parameters can be arrays, indicated  //
// comma-separated lists in the configuration file. Items in the list can be  //
// quoted, which is necessary if the item contains a comma. So that leads to  //
// the problem of how to put quotes into items. This is done by using the     //
// opposite kind of quotes. (You can use either single or double quotes.)     //
//                                                                            //
//  An example of a parameter with three values is:                           //
//   parameter1="abc,def",ghi,'jkl,mno,pqr'                                   //
//  which has the following values (angle brackets are not part of the value)://
//   parameter1[0]=<abc,def>                                                  //
//   parameter1[1]=<ghi>                                                      //
//   parameter1[2]=<jkl,mno,pqr>                                              //
//  An extreme example is:                                                    //
//   parameter2="abc'def"'"'"ghi",jkl,'"mno"pqr'"'"'rst"'                     //
//  which has the following values:                                           //
//   parameter1[0]=<abc'def"ghi>                                              //
//   parameter1[1]=<jkl>                                                      //
//   parameter1[2]=<"mno"pqr'rst">                                            //
//                                                                            //
//  Multi-valued Parameters may use continuation lines. A line is continued   //
// if it ends with a comma. Any whitespace at the beginning of the next line  //
// is removed, and the line is appended to the first line. You can have as    //
// many continuation lines as you want, as long as the combined length of all //
// the appended lines (minus their beginning whitespace and line terminators) //
// does not exceed 32768 bytes. This is all handled before this method is     //
// called, so all we will see is a very long line.                            //
// -------------------------------------------------------------------------- //
func (p *Parameter) SetValue(valuestr string, quote byte) error{
  // ---------------------------------- //
	// Clear any old values if they exists.
	// ---------------------------------- //
	if p.values!=nil{                     // Any old values?
	  p.values=p.values[:0]               // Yes, clear slice for reuse.
		p.quotes=p.quotes[:0]               // and clear quotes too.
	}                                     // Done clearing old values.
	if p.value!=""{                       // Any old value?
	  p.value=p.value[:0]                 // Yes, clear slice for reuse.
	}                                     // Done clearing old value.
	var curr string                       // Where to store the current value.
  inquote:=false                        // Are we in a quote?
  q:=rune(quote)                        // The quote character.
	for _,b:=range valuestr{              // For each byte in the value string...
	  switch{                             // Act according to the character in b.
		  case b==q:                        // Is it a quote?
			  inquote=!inquote                // Yes, toggle the inquote flag.
			case b==','&&!inquote:            // Is it a comma and not in a quote?
			  field:=strings.TrimSpace(curr)  // Yes, trim the current value.
				p.values=append(p.values, field)// Append the value.
				p.quotes=append(p.quotes,quote) // Append the quote.
				curr=curr[:0]                   // Clear the current value.
				p.n++                           // We have a new value.
		default:                            // Else, just append the byte to the current value.
		  curr+=string(b)                   // Append the byte to the current value.
		}                                   // Done acting according to the byte.
	}                                     // Done processing the value string.
	// ---------------------------------- //
	// Now process the last field, if any exists.
	// ---------------------------------- //
	if len(curr)>0{                       // Any value left?
	  field:=strings.TrimSpace(curr)      // Trim the current value.
		p.values=append(p.values,field)     // Append the value.
		p.quotes=append(p.quotes,quote)     // Append the quote.
		p.n++                               // We have a new value.
	}                                     // Done checking for last value.
	return nil                            // Return nil if we got here.
}                                       // ------------ SetValue ----------- //
// -------------------------- // SetValuePtr // ----------------------------- //
// Replace the Parameter object values with this value and quote. Whatever the
// number of values the Parameter had before this callm it will have one value
// after this call.
// -------------------------------------------------------------------------- //
func (p *Parameter) SetValuePtr(value string,quote byte) error{
  // Clear old values (keeping capacity) and append new ones.
	p.values=append(p.values[:0],value)
	p.quotes=append(p.quotes[:0],quote)
	p.n=uint(len(p.values))               // We have this many values.
	return nil                            // Return no error if we got here.
}                                       // ----------- SetValuePtr ---------- //
// -------------------- // SetValuePtrOnIndex // ---------------------------- //
// Set a value of a multi-valued Parameter. This method cannot change the number
// of values in the multi-valued Parameter, so the value must have already
// existed before the call. If the index points to a value beyond the currently
// existing number of values this method will fail.
// -------------------------------------------------------------------------- //
func (p *Parameter) SetValuePtrOnIndex(i uint,value string,quote byte) error{
  if int(i)<0{
    return fmt.Errorf("index %d out of range", i)
  }
	// Grow the valies slice if needed.
	if int(i)>=len(p.values){             // Is the index out of range?
	  newlen:=int(i)+1                    // Yes, grow the slice.
		tmp:=make([]string,newlen)          // Allocate a new slice.
		copy(tmp,p.values)                  // Copy the old values.
		p.values=tmp                        // Set the new values.
	}                                     // Done checking for out of range.
	p.values[i]=value                     // Set the value.
	// Grow the quotes slice if needed.
	if int(i)>=len(p.quotes){             // Is the index out of range?
	  tmp:=make([]byte,int(i)+1)          // Yes, grow the slice.
		copy(tmp,p.quotes)                  // Copy the old quotes.
		p.quotes=tmp                        // Set the new quotes.  
	}                                     // Done checking for out of range.
	p.quotes[i]=quote                     // Set the quote.
	p.n=uint(len(p.values))               // We have this many values.
	return nil                            // Return no error if we got here.   
}                                       // -------- SetValuePtrOnIndex ------ //
func (p *Parameter)GetQuote(i uint) (byte,error){
  if int(i)>len(p.quotes){              // Subscript out of range?
	  return 0, fmt.Errorf("index %d out of range", i)// Yes, error out.
	}                                     // Done checking for out of range subscript.
	if int(i)<len(p.quotes)&&p.quotes[i]!=0{// Is it a quote?
	  return p.quotes[i],nil              // Yes, return the quote.
	}                                     // Done checking for quote.
  v:=p.values[i]                        // Get the value.
	switch{                               // Act according to the value.
	  case strings.ContainsRune(v,'"'):   // Is it a double quote?
		  return '\'',nil                   // Yes, return a single quote.
		case strings.ContainsAny(v,"',"):   // Is it a single quote or comma?
		  return '"',nil                    // Yes, return a double quote.
		default:                            // Else it is neither.
		  return 0,nil                      // So return nothing.
	}                                     // Done acting according to the value.
}                                       // ----------- GetQuote ------------ //
// ----------------------------- // Print // -------------------------------- //
// Print the object to the stream output.
// -------------------------------------------------------------------------- //
func (p *Parameter) Print(w io.Writer) (int64,error){
  var n int64                          // Number of bytes written.
	for c:=p.comments;c!=nil;c=c.next{   // For each comment listed.
	  if !c.IsImported() || c.IsImportStatement(){
		  k,err:=w.Write([]byte(c.value+"\n"))// Buffer to the stream writter.
			n+=int64(k)                       // Add the number of bytes written.
			if err!=nil{                      // Any error?
			  return n,err                    // Yes, return the error.
			}                                 // Done printing the comment.
		}                                   // Done checking for import statement.
	}                                     // Done iterating comment list.
	var sb strings.Builder                // Where to store the string.
	sb.WriteString(p.name)                // Write the name to the string.
	if len(p.values)>0{                   // Any values to print?
	  sb.WriteString("=")                 // Yes, append the '=' sign.
		for i,v:=range p.values{            // For each value...
		  if i>0{                           // First value?
			  sb.WriteByte(',')               // No, append a comma to multivalued parameter.
			}                                 // Done checking for first value.
			q:=p.quotes[i]                    // Get the quote for this value.
			if q!=0{                          // Any quotes?
			  sb.WriteByte(q)                 // Yes, append the quote.
			}                                 // Done checking for quotes.
			sb.WriteString(v)                 // Append the value.
			if q!=0{                          // Any quotes?
			  sb.WriteByte(q)                 // Yes, append the quote.
			}                                 // Done checking for quotes.
		}                                   // Done iterating values.
	}                                     // Done checking for values.
	sb.WriteByte('\n')                    // Append a newline to the string.
	k,err:=w.Write([]byte(sb.String()))   // Write the string to the stream.
	return n+int64(k),err                 // Return # of byte written/error if any.
}                                       // ------------- Print ------------- //
// ============================== // Section // ============================= //
// A class to store an entire section of a configuration file.                //
// ========================================================================== //
func NewSection(cfg *Configuration, name string, comments *Comment, imported bool) *Section{
  return &Section{
	  cfg: cfg,                           // Configuration object.
		name: name,                         // Name of the section.
		comments: comments,                 // Comments for the section.
		isimported: imported,               // Importation flag.
	}                                     // Done creating section object.                      
}                                       // ----------- NewSection ----------- //
func NewSection2(cfg *Configuration, name string, prev *Section, comments *Comment, imported bool) *Section{
  if prev!=nil{                         // Any previous section?
	  prev.next=NewSection(cfg,name,comments,imported)// Yes, this object is the next.
	}                                     // Done checking for previous section.
  s:= &Section{
	  cfg: cfg,                           // Configuration object.
		name: name,                         // Name of the section.
		comments: comments,                 // Comments for the section.
		isimported: imported,               // Importation flag.	
	}                                     // Done creating section object.
	if prev!=nil{                         // Any previous section?
    prev.next=s                         // Yes, make us its next section.	
	}                                     // Done checking for previous section.
	return s                              // Return the new section object.
}                                       // ----------- NewSection2 ---------- //
// ------------------------------------ //
// Section getters and setters.
// ------------------------------------ //
func (s *Section) GetPathname() string { return s.cfg.GetPathname() }
func (s *Section) GetDirectory() string { return s.cfg.GetDirectory() }
func (s *Section) GetFilename() string { return s.cfg.GetFilename() }
func (s *Section) GetName() string { return s.name }
func (s *Section) GetComments() *Comment { return s.comments }
func (s *Section) GetNext() *Section { return s.next }
func (s *Section) SetNext(p *Section){ if s!=nil{ s.next=p}}
func (s *Section) GetFirst() *Parameter { return s.first }
func (s *Section) GetLast() *Parameter { return s.last }
func (s *Section) GetFirstSection() *Section { return s.firstSection }
func (s *Section) GetNParameters() uint { return s.nParameters }
func (s *Section) GetNParents() uint { return s.nParents }
func (s *Section) GetNSections() uint { return s.nSections }
func (s *Section) IsImported() bool { return s.isimported }
func (s *Section) GetSelectedParameterName() string{
  if s.current!=nil{  return s.current.GetName() }
	return ""
}
func (s *Section) GetSelectedParameter() *Parameter{ return s.current }
func (s *Section) SelectFirstParameter(){ s.current=s.first }
func (s *Section) SelectParameter(p *Parameter) { s.current=p }
func (s *Section) SelectParameterByName(name string) error{
  s.current=s.FindParameter(name,true)
	if s.current==nil{
	  return fmt.Errorf("parameter %s not found", name)
	}
	return nil
}
func (s *Section) FindFirstParameter() *Parameter{
  return s.GetFirst()                   // Return the first parameter in the list.
}
func (s *Section) FindSection(name string) *Section{
  for p:=s.firstSection;p!=nil;p=p.GetNext(){// For each section in the list...
	  if strings.EqualFold(p.GetName(),name){// Is it the section we are looking for?
		  return p                          // Yes, we found the section, return it.
		}                                   // Done checking for section.
	}                                     // Done iterating through sections.
	return nil                            // No section found, return nil.
}                                       // ----------- FindSection ---------- //
func (s *Section) GetParameter(name string, searchParents bool) *Parameter{
  return s.FindParameter(name,searchParents) // Find the parameter by name.
}                                       // ----------- GetParameter --------- //
// -------------------------- // FindParameter // --------------------------- //
//  Find a Parameter from this section by name. Return nullptr if there is no //
// Parameter with a matching name. Searches parent Sections if it is not      //
// found.                                                                     //
//  This method should not be mixed with calls to FindFirstParameter() and    //
// FindNextParameter(). That keeps a current Parameter pointer that has       //
// nothing to do with this method.                                            //
// Note: This method is recursive. If it does not find a Parameter, and this  //
//       Section has a parent, it will call itself to search the parent.      //
//       It will continue up the hierarchy of parents until it either finds   //
//       the Parameter, or until it reaches the root Section.                 //        
// -------------------------------------------------------------------------- //
func (s *Section) FindParameter(name string, searchParents bool) *Parameter{
  p:=s.first                            // The first parameter in the list.
	for ;p!=nil;p=p.next{                 // For each parameter in the list...
	  if strings.EqualFold(p.GetName(),name){// Did we find it?
		  break                             // Yes, break out of the loop.
		}                                   // Done checking for parameter. 
	}                                     // Done iterating through the list.
	// ---------------------------------- //
	// We did not find the parameter in this section. But we have parent and we 
	// are allowed to seach in those sections for the parameter, so we will do 
	// that next.
	// ---------------------------------- //
	if (p==nil&&searchParents&&s.nParents!=0){
	  for i:=0;i<int(s.nParents);i++{     // For each parent...
		  //parent:=s.parents[i]              // Get the parent section.
			//parentName:=parent.GetName()      // Get the parent name.
			p=s.parents[i].FindParameter(name,true)// Search the parent.
			if p!=nil { break }               // If we found it, break from the loop.
		}                                   // Done iterating through the parents.
	}                                     // Done checking for parent sections.
	return p                              // Return what we found.
}                                       // ---------- FindParameter --------- //
// ------------------------ // FindNextParameter // ------------------------- //
//  Find the next Parameter in this section. Return nullptr if we are already //
// at the last Parameter, or if there are no Parameters.                      //
//  This is used when a section just has a list of values but no parameter    //
// names.                                                                     //
// -------------------------------------------------------------------------- //
func (s *Section) FindNextParameter() *Parameter{
  if s.current!=nil{                    // Any current parameter?
	  s.current=s.current.GetNext()       // Yes, get the next parameter.
	}                                     // Done checking for current parameter.
	return s.current                      // Return what we currently have.
}                                       // --------- FindNextParameter ------ //
// ------------------------- // ClearParameters // -------------------------- //
// Erase all Parameter objects from this section.                             //
// -------------------------------------------------------------------------- //
func (s *Section) ClearParameters() error{
  for p:=s.first;p!=nil;p=p.GetNext(){  // For each parameter in our list...
	  s.first,s.last,s.current=nil,nil,nil// Clear the list.
		s.nParameters=0                     // Reset our count to 0.
	}                                     // Done iterating through the list.
	return nil                            // Always successful.
}                                       // --------- ClearParameters ------- //
func (s *Section) GetParent(n uint) *Section{
  if s.nParents==0{                     // Any parents?
	  return nil                          // No, return nil.
	}else{                                // Else we have parents!
	  return s.parents[n]                 // Return the requested parent.
	}                                     // Done checking for parents.
}                                       // ----------- GetParent ------------ //
func (s *Section) GetParentName (n uint) string{
  if s.nParents==0{                     // Any parents?
	  return ""                           // No, return empty string.
	}else{                                // Else we have parents!
	  return s.parentNames[n]             // Return the requested parent name.
	}                                     // Done checking for parents.
}                                       // ----------- GetParentName --------- //
func (s *Section) RemoveMissingParent (i uint){
  for j:=i;j<s.nParents;j++{            // For each parent...
	  s.parents[i]=s.parents[j]           // Move the next parent into this one.
	}                                     // Done moving parents.
	s.nParents--                          // Decrement the number of parents.                              
}                                       // ------ RemoveMissingParent ------ //
// -------------------------- // SetParentNames // -------------------------- //
// SetParentNames stores the literal parent names that appear after the ':'
// in a section header.  This is normally only called by ReadFile() in the
// Configuration object.
//
// Example header:
//   [child:parent1,parent2]
//
// After the call:
//
//   s.nParents    == 2
//   s.parentNames == []string{"parent1", "parent2"}
//   s.parents     == make([]*Section, 2)   // filled in ResolveParents()
// -------------------------------------------------------------------------- //
func (s *Section) SetParentNames(list string){
    // -------------------------------- //
	  // If this method is called more than once, wipe the old data first.
	  // -------------------------------- //
  if s.parentNames!=nil{                // Do we have old parent names already?
	  s.parentNames=nil                   // Yes, clear the slice.
		s.parents=nil                       // and clear the parents too.
		s.nParents=0                        // Reset the number of parents.
	}                                     // Done clearing old parent names.
	list=strings.TrimSpace(list)          // Remove leading / trailing whitespace.
	if list==""{                          // Did they give use any parent names?
	  return                              // No, nothing to do, return.
	}                                     // Done checking for empty list.
    // -------------------------------- //
    // We have been given a containing a comma-separated list of parents.
    // Because this is Go, and we are not dealing with C-style strings.
		// We simply have to split the list by the commas, and append them.
    // -------------------------------- //
	parts:=strings.Split(list,",")        // Split the list by commas.
	for i:=range parts{                   // For each part in the list...
	  parts[i]=strings.TrimSpace(parts[i])// Trim whitespace from each part.
	}                                     // Done trimming whitespace from parts.
	s.parentNames=parts                   // Store the parent names.
	s.nParents=uint(len(parts))           // Set the number of parents.
	// ---------------------------------- //
	// No we have the parent names, the number of parents. We now make room
	// for however may parents we have. We don't know the Section yet, but that
	// is okay, because we fill that in later when we resolve the parents by 
	// calling resolveParent() in the ReadFile() method.
	// ---------------------------------- //
	s.parents=make([]*Section,s.nParents) // Allocate space for the parent sections.
}                                       // --------- SetParentNames --------- //
// This should only be called by ReadFile.
func (s *Section) SetParentSection(i uint, p *Section){
  s.parents[i]=p                        // Set the parent section.
}                                       // -------- SetParentSection -------- //
// ------------------------------ // Append // ------------------------------ //
//  Append a new Section to the list; return a pointer to that new Section.   //
// *** old -- now puts the new section after this one. *** 
// Can be called for any member of the list, but it always puts the new       //
// Section at the end of the list.  *** old (see commented-out lines) ***     //
// -------------------------------------------------------------------------- //
func (s *Section) Append(name string, imported bool) *Section{
  newsect:=NewSection(s.cfg,name,s.comments,imported)// A new Section object.
	if s!=nil{                            // Any previous section to append to?
	  s.next=newsect                      // Yes, append new section to the list.
	}                                     // Done checking for previous section.
	return newsect                        // Return the new Section object.
}                                       // ------------ Append ------------- //
// ------------------------------- // Append2 // ----------------------------- //
//  Append a parameter to a Section. This is mainly used for test programs.   //
// The Section object makes a copy of the Parameter, so it is responsible for //
// deleting that copy.                                                        //
// -------------------------------------------------------------------------- //
func (s *Section) Append2(p *Parameter) *Parameter{
  q:=CopyParameter(p)                   // Create a Parameter object copy.
	if s.first==nil{                      // Any parameter in the list?
	  s.first=q                           // No, this is the first one.
	} else{                               // Else we have parameters in the list.
	  s.last.SetNext(q)                   // Place q next to the last one.
	}                                     // Done checking if we had a list.
	s.last=q                              // But now q is the last one.
	s.nParameters++                       // Always keep track of # of Parameters.
	return q                              // Return the appended Parameter object.                                     
}                                       // ------------ Append2 ------------ //
// --------------------------- // AppendParameter // ------------------------ //
//  Append a new Parameter to a Section. This is used when reading the file,  //
// so it is allowed to set comments. If Go did not collect the garbage, we would
// have to deal with deleting the Parameter object we just created in a 
// destructor. Keep that in mind, since memory is typically leaky and we don't 
// want to leak memory. But Go does garbage collection so it is not a problem.
// -------------------------------------------------------------------------- //
func (s *Section) AppendParameter(name, valuestr string, comments *Comment,imported bool) *Parameter{
  p:=NewParameter(name,valuestr,comments,imported)// A new Parameter object.
	if s.first==nil{                      // Any parameter in the list?
	  s.first=p                           // No this is the first one.
	} else{                               // Else we have parameters in the list.
	  s.last.SetNext(p)                   // Place p next to the last one.
	}                                     // Done checking if we had a list.
	s.last=p                              // But now p is the new last one.
	s.nParameters++                       // Always keep track of # of Parameters.
	return p                              // Return the appended Parameter object.
}                                       // --------- AppendParameter -------- //
// ----------------------------- // AppendSection // ------------------------ //
// Append a Section to this Section. This will be a copy of the Section stored
// somewhere else but with different links.
// -------------------------------------------------------------------------- //
func (s *Section) AppendSection(name string, imported bool){
  newsect:=NewSection(s.cfg,name,s.comments,imported)// A new section object.
	if s.firstSection==nil{               // Any section in the list?
	  s.firstSection=newsect              // No, this is the first one.
	} else{                               // Esle we have sections in the list.
	  s.lastSection.SetNext(newsect)      // Place newsect next to the last one.
	}                                     // Done checking for previous section.
	s.lastSection=newsect                 // But now newsect is the last one.
	s.nSections++                         // Always keep track of # of sections.
}                                       // --------- AppendSection ---------- //
// ------------------------------ // SetValue // ---------------------------- //
//  Set the value of an existing Parameter. This is used by the application   //
// to change parameters that can be modified. The application is not allowed  //
// to change the comments.                                                    //
//  If the parameter is not found in this Section, the parents are not        //
// searched. This keeps you from having one section of code change a          //
// Parameter that is used by another section of code. If you really want to   //
// change a value in a parent, you can call GetParent() after this fails and  //
// keep trying to go up the hierarchy until you find the parameter.           //
// -------------------------------------------------------------------------- //
func (s *Section) SetValue(name, value string, quote byte) error{
  p:=s.FindParameter(name,false)        // Find the parameter in this section.
	if p!=nil{                            // Did we find it?
	  return p.SetValue(value,quote)      // Yes, set the value.
	}                                     // Done checking if we found it.
	return fmt.Errorf("parameter %s not found in section %s", name, s.name)// No, return error.
}                                       // ----------- SetValue ------------ //
func (s *Section) SetValuePtr(name,value string, quote byte) error{
  p:=s.FindParameter(name,false)        // Find the parameter in this section.
	if p!=nil{                            // Did we find it?
	  return p.SetValuePtr(value,quote)   // Yes, set the value.
	}                                     // Done checking if we found it.
	return fmt.Errorf("parameter %s not found in section %s", name, s.name)// No, return error.
}                                       // ----------- SetValuePtr --------- //
func (s *Section) SetValuePtrOnIndex(name,value string, i uint, quote byte) error{
  p:=s.FindParameter(name,false)          // Find the parameter in this section.
	if p!=nil{                            // Did we find it?
	  return p.SetValuePtrOnIndex(i,value,quote)// Yes, set the value.
	}                                     // Done checking if we found it.
	return fmt.Errorf("parameter %s not found in section %s", name, s.name)// No, return error.
}                                       // --------- SetValuePtrOnIndex ----- //

// ---------------------------- // GetNValues // ---------------------------- //
// Return the number of values in the given Parameter of this Section.        //
// Note: If the Parameter is not found, or if none is given and none is       //
//       selected, then this method just returns zero. This is the same       //
//       result as if the Parameter exists but has no values.                 //
// -------------------------------------------------------------------------- //
func (s *Section) GetNValues(name string) uint{
  if name!=""{                          // Was a name given?
	  p:=s.FindParameter(name,true)       // Yes, find the parameter.
		if p!=nil{                          // Did we find it?
		  return p.GetNValues()             // Yes, return its value content.
		}                                   // Done checking for parameter.
	} else if s.current!=nil{             // No name given, is one selected?
	  return s.current.GetNValues()       // Return count for selected parameter.
	}                                     // Done checking for selected parameter.
	return 0                              // Otherwise return 0.
}                                       // ----------- GetNValues ----------- //
// ---------------------------- // GetValues // ----------------------------- //
//  Get a pointer to all of the values for the given Parameter. Return        //
// nullptr if there is no Parameter with that name, or if the Parameter has   //
// no value. It also searches parent Sections if the Parameter is not found   //
// in this Section.                                                           //
// -------------------------------------------------------------------------- //
func (s *Section) GetValueArray(name string) []string{
  p:=s.FindParameter(name,true)         // Find the parameter in this section.
	if p!=nil{                            // Did we find its the parameter?
	  return p.GetValueArray()            // Yes, return its value array.
	}                                     // Done checking for parameter.
	return nil                            // Otherwise return nil.
}                                       // ----------- GetValues ------------ //
func (s *Section) GetValues(name string) string{
  p:=s.FindParameter(name,true)         // Find the parameter in this section.
	if p!=nil{                            // Did we find its the parameter?
	  return p.GetValues()                // Yes, return its values.
	}                                     // Done checking for parameter.
	return ""                             // Otherwise return empty string.
}                                       // ----------- GetValue ------------ //
// ----------------------------- // GetValue // ----------------------------- //
//  Get a value for the Parameter. Return nullptr if there is no Parameter    //
// with that name, or if the Parameter has no value. It also searches parent  //
// Sections if the Parameter is not found in this Section.                    //
// -------------------------------------------------------------------------- //
func (s *Section) GetValue(name string, i uint) string{
  p:=s.FindParameter(name,true)         // Find the parameter in this section.
	if p!=nil{                            // Did we find its the parameter?
	  return p.GetValue(i)                // Yes, return its value.
	}                                     // Done checking for parameter.
	return ""                             // Otherwise return empty string.
}                                       // ----------- GetValue ------------ //

// --------------------------- // GetValue // ------------------------------- //
// Get the value from a given Parameter pertaining to this section.
// -------------------------------------------------------------------------- //

// ----------------------- Byte values (character values) ------------------- //
func (s *Section)	GetValueByte(name string, dest *byte) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{
	  return fmt.Errorf("can't decode empty \"value\" to byte")
	}
	return s.scanValue(p,"%c",dest) 
}
func (s *Section)	GetValueByteByIndex(name string,i uint,dest *byte) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	}
	return s.scanValueByIndex(name,int(i),"%c",dest)
}

// -------------------------- Times and durations -------------------------- //
func (s *Section)	GetValueTimespec(name string, dest *unix.Timespec) error{
  p:=s.GetValue(name,0)                 
  if len(p)==0{                         // Where we given a value to decode?
	  return fmt.Errorf("can't decode empty \"value\" to unix.Timespec")// No, that's bad.
	}                                     // Done checking for empty value.
	t,err:=time.Parse(time.RFC3339,p) // Parse the value as a time.
	if err!=nil{                          // Any error parsing the time?
	  return fmt.Errorf("can't decode \"%s\" to unix.Timespec: %v", p, err)
	}                                     // Done checking for parse error.
	unix.NsecToTimespec(t.UnixNano()) 	  // Convert the time to a unix.Timespec.
	*dest=unix.NsecToTimespec(t.UnixNano()) // Set the destination time to the parsed time.
	return nil                            // Return nil if we got here.
}
func (s *Section)	GetValueTimespecByIndex(name string,i uint,dest *unix.Timespec) error{
  p:=s.GetValue(name,i)                 // Get the value for the given name.
	if i>=uint(len(p)){                   // Is the index out of range?
	  return fmt.Errorf("index %d out of range", i)// Yes, panic.
	}                                     // Done checking for out of range index.
	q:=p                                  // Get the value at the index.
	t,err:=time.Parse(time.RFC3339,q)     // Parse the value as a time.
	if err!=nil{                          // Any error parsing the time?
	  return fmt.Errorf("can't decode \"%s\" to time.Time: %v", q, err)
	}                                     // Done checking for parse error.
	unix.NsecToTimespec(t.UnixNano()) 	  // Convert the time to a unix.Timespec.
	*dest=unix.NsecToTimespec(t.UnixNano()) // Set the destination time to the parsed time.
	return nil                            // Return nil if we got here.
}
func (s *Section)	GetValueDuration(name string, dest *time.Duration) error{
  p:=s.GetValue(name,0)                 // Get the value for the given name.
	if len(p)==0{                         // Where we given a value to decode?
	  return fmt.Errorf("can't decode empty \"value\" to time.Duration")// No, that's bad.
	}                                     // Done checking for empty value.
	d,err:=time.ParseDuration(p)          // Parse the value as a duration.
	if err!=nil{                          // Any error parsing the duration?
	  return fmt.Errorf("can't decode \"%s\" to time.Duration: %v", name, err)
	}                                     // Done checking for parse error.
	*dest=d                               // Set the destination duration to the parsed duration.
	return nil                            // Return nil if we got here.  
}
func (s *Section)	GetValueDurationByIndex(name string,i uint,dest *time.Duration) error{
  p:=s.GetValue(name,i)						      // Get the value for the given name.
	if i>=uint(len(p)){                   // Is the index out of range?
	  return fmt.Errorf("index %d out of range", i)// Yes, panic.
	}                                     // Done checking for out of range index.
	q:=p                                  // Get the value at the index.
	d,err:=time.ParseDuration(q)          // Parse the value as a duration.
	if err!=nil{                          // Any error parsing the duration?
	  return fmt.Errorf("can't decode \"%s\" to time.Duration: %v", q, err)
	}                                     // Done checking for parse error.
	*dest=d                               // Set the destination duration to the parsed duration.
	return nil                            // Return nil if we got here. 
}
// Time since epoch
func (s *Section)	GetValueTime(name string, dest *time.Time) error{
  p:=s.GetValue(name,0)                 // Get the value for the given name.
	if len(p)==0{                         // Where we given a value to decode?
	  return fmt.Errorf("can't decode empty \"value\" to time.Time")// No, that's bad.
	}                                     // Done checking for empty value.
	t,err:=time.Parse(time.RFC3339,p)     // Parse the value as a time.
	if err!=nil{                          // Any error parsing the time?
	  return fmt.Errorf("can't decode \"%s\" to time.Time: %v", name, err)
	}                                     // Done checking for parse error.
	*dest=t                               // Set the destination time to the parsed time.
	return nil                            // Return nil if we got here.	
	}
func (s *Section)	GetValueTimeByIndex(name string, i uint,dest *time.Time) error{
  p:=s.GetValue(name,i)                 // Get the value for the given name.
	if i>=uint(len(p)){                   // Is the index out of range?
	  return fmt.Errorf("index %d out of range", i)// Yes, panic.
	}                                     // Done checking for out of range index.
	q:=p                                  // Get the value at the index.
	t,err:=time.Parse(time.RFC3339,q)     // Parse the value as a time.
	if err!=nil{                          // Any error parsing the time?
	  return fmt.Errorf("can't decode \"%s\" to time.Time: %v", q, err)
	}                                     // Done checking for parse error.
	sec,nsec:=t.UnixNano()/1e9,t.UnixNano()%1e9 // Get seconds and nanoseconds.
	*dest=time.Unix(sec,nsec).In(t.Location())        
	return nil                            // Return nil if we got here. 
}

// -------------------------- Signed Integers ------------------------------- //
func (s *Section)	GetValueInt(name string, dest *int) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to int")
	}                                     
	return s.scanValue(p,"%d",dest)     
}
func (s *Section)	GetValueIntByIndex(name string,i uint,dest *int) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	} 
	return s.scanValueByIndex(name,int(i),"%d",dest) 
}
func (s *Section)	GetValueInt8(name string, dest *int8) error{
	p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to int8")
	}                                     
	return s.scanValue(p,"%d",dest) 
}
func (s *Section)	GetValueInt8ByIndex(name string,i uint,dest *int8) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	}                                     
	return s.scanValueByIndex(name,int(i),"%d",dest) 
}
func (s *Section)	GetValueInt16(name string, dest *int16) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to int16")
	}                                     
	return s.scanValue(p,"%d",dest) 
}
func (s *Section)	GetValueInt16ByIndex(name string,i uint,dest *int16) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	}                                  
	return s.scanValueByIndex(name,int(i),"%d",dest) 
}
func (s *Section)	GetValueInt32(name string, dest *int32) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to int32")
	}                                     
	return s.scanValue(p,"%d",dest) 
}
func (s *Section)	GetValueInt32ByIndex(name string,i uint,dest *int32) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	}                                
	return s.scanValueByIndex(name,int(i),"%d",dest) 
}
func (s *Section)	GetValueInt64(name string, dest *int64) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to int64")
	}                                     
	return s.scanValue(p,"%d",dest) 
}
func (s *Section)	GetValueInt64ByIndex(name string,i uint,dest *int64) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	}                                 
	return s.scanValueByIndex(name,int(i),"%d",dest) 
}

// ------------------- Unicode, binary and hex values ----------------------- //
func (s *Section) GetValueRune(name string, dest *rune) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to rune")
	}                                     
	return s.scanValue(p,"%c",dest) 
}
func (s *Section)	GetValueRuneByIndex(name string,i uint,dest *rune) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	}                                    
	return s.scanValueByIndex(name,int(i),"%c",dest) 
}
func (s *Section)	GetValueBinary(name string, dest *string) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to binary string")
	}                                     
	return s.scanValue(p,"%b",dest) 
}
func (s *Section)	GetValueHex(name string, dest *string) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to hex string")
	}                                     
	return s.scanValue(p,"%x",dest) 
}
func (s *Section)	GetValueOctal(name string, dest *string) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to octal string")
	}                                     
	return s.scanValue(p,"%o",dest) 
}

// ----------------------- Unsigned integers -------------------------------- //
func (s *Section)	  GetValueUint(name string, dest *uint) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to uint")
	}                                     
	return s.scanValue(p,"%d",dest) 
}
func (s *Section)	GetValueUintByIndex(name string,i uint,dest *uint) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	}                                     
	return s.scanValueByIndex(name,int(i),"%d",dest) 
}
func (s *Section)	GetValueUint8(name string, dest *uint8) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to uint8")
	}                                     
	return s.scanValue(p,"%d",dest) 
}
func (s *Section)	GetValueUint8ByIndex(name string,i uint,dest *uint8) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	}                                    
	return s.scanValueByIndex(name,int(i),"%d",dest) 
}
func (s *Section)	GetValueUint16(name string, dest *uint16) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to uint16")
	}                                     
	return s.scanValue(p,"%d",dest) 
}
func (s *Section)	GetValueUint16ByIndex(name string,i uint, dest *uint16) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	}                                     
	return s.scanValueByIndex(name,int(i),"%d",dest) 
}
func (s *Section)	GetValueUint32(name string, dest *uint32) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to uint32")
	}                                     
	return s.scanValue(p,"%d",dest) 
}
func (s *Section)	GetValueUint32ByIndex(name string,i uint,dest *uint32) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	}                                 
	return s.scanValueByIndex(name,int(i),"%d",dest) 
}
func (s *Section)	GetValueUint64(name string, dest *uint64) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to uint64")
	}                                     
	return s.scanValue(p,"%d",dest) 
}
func (s *Section)	GetValueUint64ByIndex(name string,i uint,dest *uint64) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	}                                    
	return s.scanValueByIndex(name,int(i),"%d",dest) 
}

// ------------------------- Floating point values -------------------------- //
func (s *Section)	GetValueFloat32(name string, dest *float32) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to float32")
	}                                     
	return s.scanValue(p,"%f",dest)     
}
func (s *Section)	GetValueFloat32ByIndex(name string,i uint,dest *float32) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	}                                  
	return s.scanValueByIndex(name,int(i),"%f",dest) 
}
func (s *Section)	GetValueFloat64(name string,dest *float64) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to float64")
	}                                     
	return s.scanValue(p,"%f",dest)     
}
func (s *Section)	GetValueFloat64ByIndex(name string,i uint,dest *float64) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	} 
	return s.scanValueByIndex(name,int(i),"%f",dest)
}

// Floating point values with precision
func (s *Section)	GetValuePrecisionFloat32(name,precision string,dest *float32) error{
  p:=s.GetValue(name,0)
	if len(p)==0{
	  return fmt.Errorf("can't decode empty \"value\" to float32 with precision %s", precision)
	}
	return s.scanValue(p,fmt.Sprintf("%%.%sf", precision),dest)
}
func (s *Section)	GetValuePrecisionFloat32ByIndex(name string,i uint,precision string,dest *float32) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	} 
	return s.scanValueByIndex(name,int(i),fmt.Sprintf("%%.%sf", precision),dest)
}
func (s *Section)	GetValuePrecisionFloat64(name string,value,precision string,dest *float64) error{
  p:=s.GetValue(name,0)
	if len(p)==0{
	  return fmt.Errorf("can't decode empty \"value\" to float64 with precision %s", precision)
	}
	return s.scanValue(p,fmt.Sprintf("%%.%sf", precision),dest)
}
func (s *Section)	GetValuePrecisionFloat64ByIndex(name string,i uint,precision string,dest *float64) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	} 
	return s.scanValueByIndex(name,int(i),fmt.Sprintf("%%.%sf", precision),dest)
}

// --------------------------- Complex numbers ------------------------------ //
func (s *Section)	GetValueComplex64(name string,dest *complex64) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to complex64")
	}                                     
	return s.scanValue(p,"%v",dest)     
}
func (s *Section)	GetValueComplex64ByIndex(name string,i uint,dest *complex64) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	}                                   
	return s.scanValueByIndex(name,int(i),"%v",dest) 
}
func (s *Section)	GetValueComplex128(name string,dest *complex128) error{
  p:=s.GetValue(name,0)                 
	if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to complex128")
	}                                     
	return s.scanValue(p,"%v",dest)     
}
func (s *Section)	GetValueComplex128ByIndex(name string,i uint,dest *complex128) error{
  if len(name)==0{
	  return fmt.Errorf("name cannot be empty")
	}                                    
	return s.scanValueByIndex(name,int(i),"%v",dest) 
}

// --------------------------- Scientific notation -------------------------- //
func (s *Section)	GetValueSI(name string,dest *string) error{
  p:=s.GetValue(name,0)
	if len(p)==0{
	  return fmt.Errorf("can't decode empty \"value\" to scientific notation")
	}
	return s.scanValue(p,"%e",dest)
}
// ----------------------------- // GetValueBool // ------------------------- //
//  Get the value of a bool parameter. If values are given for true and       //
// false, then the result must match one or the other. If you give either a   //
// true or false value, but not the other, then the result will always be     //
// false. If tval and fval are the same, then the result will always be true. //
//  If the value does not match either of the tval and fval values, the       //
// result will not be set and the method will return false.                   //
// -------------------------------------------------------------------------- //
func (s *Section) GetValueBool(name string,i uint,tval string, fval string) (result bool, err error){
  
	p:=s.GetValue(name,i)                 // Get the parameter value.
	if p!=""{                             // Did we get a value?
	  if tval==""&&fval==""{              // No values given?
		  if isTrue(p){                     // Is it "true"?
			  result=true                     // Yes, set result to true.
			}else if isFalse(p){              // Is it "false"?
			  result=false                    // Yes, set result to false.
			} else{                           // Else we dont know what it is.
			  result=false                    // So make it false.
				err=fmt.Errorf("value %s is not a boolean", p)// Store error.
			}                                 // Done checking if could decode p.
		}else if tval!=""&&fval!=""{        // Caller gave possible values?
		  if strings.Contains(tval,p){      // Is it a true value?
			  result=true                     // Yes set result to true.
			}else if strings.Contains(fval,p){// Is it a false value?
			  result=false                    // Yes set result to false.
			} else{                           // Otherwise we cant decode it.
			  result=false                    // Set result to false.
				err=fmt.Errorf("value %s is not a boolean", p)// Store error.
			}                                 // Done checking if can decode p.
		}                                   // Done checking if caller gave values.
	}                                     // Done checking for value.
  return result,err                     // Return result and error if any.
}                                       // ----------- GetValueBool --------- //
// ------------------------------ // Print // ------------------------------- //
// Write this Section object to a stream.                                     //
// -------------------------------------------------------------------------- //
func (s *Section) Print(w io.Writer) (int64,error){
  var n int64                           // Number of bytes written.
	for c:=s.comments;c!=nil;c=c.GetNext(){// For each comment listed.
	  if !c.IsImported()||c.IsImportStatement(){// Is it an import statement?
		  k,err:=w.Write([]byte(c.value+"\n"))// Buffer to the stream writter.
			n+=int64(k)                       // Add the number of bytes written.
			if err!=nil{                      // Any error?
			  return n,err                    // Yes, return the error.
			}                                 // Done printing the comment.
		}                                   // Done checking for import statement.
	}                                     // Done iterating comment list.
	// ---------------------------------- //
	// Now we need to print the section header.
	// ---------------------------------- //
	header:=s.name                        // The section name.
	if s.nParents>0{                      // Any parents?
	  header=fmt.Sprintf("%s:%s",s.name,strings.Join(s.parentNames,","))
	}                                     // Yes, append the parent names.
	/*header:=strings.Builder{}             // The section name.
	header.WriteString("[")               // Start the section header.
	header.WriteString(s.name)            // Write the section name.
	if s.nParents>0{
	  header.WriteString(":")						 // Append the colon.
		for i,p:=range s.parentNames{
		  if i>0{
			  header.WriteByte(',')
			}
			header.WriteString(p)					   // Append the parent name.
		}
	}
	header.WriteString("]\n")            // End the section header.*/
	k,err:=fmt.Fprintf(w,"[%s]\n",header) // Print the section name.
	//k,err:=io.WriteString(w,header.String()) // Print the section name.
	n+=int64(k)                           // Add the number of bytes written.
	if err!=nil{                          // Any error?
	  return n,err                        // Yes, return the error.
	}                                     // Done printing the section name.
	// ---------------------------------- //
	// Now we need to print the parameters and nested section references in order.
	// ---------------------------------- //
	for p:=s.first;p!=nil;p=p.GetNext(){  // For each parameter in our list...
	  m,err:=p.Print(w)                   // Print the parameter to the stream.
		n+=m                                // Add the number of bytes written.
		if err!=nil{                        // Any error?
		  return n,err                      // Yes, return the error.
		}                                   // Done printing the parameter.
	}                                     // Done iterating through the list.
	// ---------------------------------- //
	// Now we need to print the nested sections.
	// ---------------------------------- //
	for q:=s.firstSection;q!=nil;q=q.GetNext(){// For each nested section...
	  m,err:=q.Print(w)                   // Print the section to the stream.
		n+=m                                // Add the number of bytes written.
		if err!=nil{                        // Any error?
		  return n,err                      // Yes, return the error.
		}                                   // Done printing the section.
	}                                     // Done iterating through the nested sections.
	return n,nil                          // Return the number of bytes written and no error.
}                                       // ------------- Print ------------- //
// ----------------------------- // ScanValue // ---------------------------- //
// Scan the requested value into the destination variable using the specified
// format. The destination variable must the type of value
// being scanned. The format string must match the type of the value being
// scanned. The value is scanned from the string representation of the value
// in the configuration file. The value is converted to the type of the
// destination variable using the format string.
// -------------------------------------------------------------------------- //
func (s *Section) scanValue(value, format string, dest any) error{
  if len(value)==0{                     // Within range?
	return fmt.Errorf("The value of the parameter %s is %v",s.current.name,s.current.value)
  }                                     // Done checking for out of range.
  var verb byte                         // The format verb.
  for j:=1;j<len(format);j++{           // For each character in fmt string...
	  c:=format[j]                        // Get the j'th characer.
		// -------------------------------- //
		// We need to check if 'c' is one of the characters that can legally
		// appear in the *flags, width, precision or modifier* part of a fmt string.
		// if it is, we will skip it and continue scanning the next character.
		// -------------------------------- //
		if strings.ContainsRune("#0- +. '0123456789*",rune(c)){ continue }
		// -------------------------------- //
		// Otherwise we have reached the first character that is not a flag, digit,
		// dot or star, so by definition this is the format verb.
		// -------------------------------- //
		verb=c                              // The format verb.
		break                               // We found the verb so break from loop.
	}                                     // Done iterating through the format string.
  if verb!=0&&!verbComaptible(verb,reflect.TypeOf(dest).Elem().Kind()){
	  return fmt.Errorf("format %s is not compatible with type %s", format, reflect.TypeOf(dest).Elem().Kind())
	}                                     // Done checking for format compatibility.
  val:=reflect.ValueOf(dest).Kind()     // Get the type of the dest variable.
	if dest==nil||val!=reflect.Ptr{       // Is dest nil or not a pointer?
	return errors.New("destination must be a non-nil pointer")// Yes, return an error.
  }                                     // Done checking for nil or pointer.
  //raw:=p.GetValue(uint(i))                              
	// Scan the value into the destination variable
  _,err:=fmt.Sscanf(string(value),format,dest)
	return err                            // Return error if any.  
}
func (s *Section) scanValueByIndex(name string,i int, format string, dest any) error{
  p:=s.FindParameter(name,true)         // Find the parameter in this section.
	if p==nil{                            // Did we find the parameter?
	  return fmt.Errorf("parameter %s not found in section %s", name, s.name)// No, return error.
	}                                     // Done checking for parameter.
	if i < 0 || i >= int(s.GetNValues(name)){       // Within range?
	return fmt.Errorf("index %d out of range", i)
  }                                     // Done checking for out of range.
  var verb byte                         // The format verb.
  for j:=1;j<len(format);j++{           // For each character in fmt string...
	  c:=format[j]                        // Get the j'th characer.
		// -------------------------------- //
		// We need to check if 'c' is one of the characters that can legally
		// appear in the *flags, width, precision or modifier* part of a fmt string.
		// if it is, we will skip it and continue scanning the next character.
		// -------------------------------- //
		if strings.ContainsRune("#0- +. '0123456789*",rune(c)){ continue }
		// -------------------------------- //
		// Otherwise we have reached the first character that is not a flag, digit,
		// dot or star, so by definition this is the format verb.
		// -------------------------------- //
		verb=c                              // The format verb.
		break                               // We found the verb so break from loop.
	}                                     // Done iterating through the format string.
  if verb!=0&&!verbComaptible(verb,reflect.TypeOf(dest).Elem().Kind()){
	  return fmt.Errorf("format %s is not compatible with type %s", format, reflect.TypeOf(dest).Elem().Kind())
	}                                     // Done checking for format compatibility.
  val:=reflect.ValueOf(dest).Kind()     // Get the type of the dest variable.
	if dest==nil||val!=reflect.Ptr{       // Is dest nil or not a pointer?
	return errors.New("destination must be a non-nil pointer")// Yes, return an error.
  }                                     // Done checking for nil or pointer
	// Scan the value into the destination variable
  _,err:=fmt.Sscanf(string(s.GetValue(name,uint(i))),format,dest)
	return err                            // Return error if any.
}                                       // ------------ ScanValue ----------- //
// -------------------------- // SetValueFormat // ------------------------- //
// Set the value of a Parameter using a format string. It formats the value in
// src according to the format string and set the value of the Parameter name,
// at element i, to the formatted value.                                      //
// -------------------------------------------------------------------------- //
func (s *Section) SetValueInFormat(name string,i int,format string,src any) error{
  if i<0{                               // Were we given a valid index?
	  return fmt.Errorf("index %d must be non-negative", i)// No, return error.
	}                                     // Done checking for valid index.
	p:=s.FindParameter(name,false)        // Find the parameter in this section.
	if p==nil{                            // Did we find the parameter?
	  p=s.AppendParameter(name,"",nil,false)// No, append a new parameter.
	}                                     // Done checking for parameter.
	var valstr string                     // The formatted value to set.
	if format==""{                        // Did they give us a format?
	  valstr=fmt.Sprintf("%v",src)        // No, just use the default format.
	} else{                               // Else they gave us a format.
	  valstr=fmt.Sprintf(format,src)      // Use the format to format the value.
	}                                     // Done checking for format.
	if i>=int(p.GetNValues()){            // Is the index out of range?
	  newlen:=i+1                         // Yes, we need to grow the values slice.
		tmpval:=make([]string,newlen)       // Make a new slice of strings.
		copy(tmpval,p.values)               // Copy the old values to the new slice.
		p.values=tmpval                     // Set the new values slice.
		tmpquo:=make([]byte,newlen)         // Make a new slice of quotes.
		copy(tmpquo,p.quotes)               // Copy the old quotes to the new slice.
		p.quotes=tmpquo                     // Set the new quotes slice.
	}                                     // Done checking for index out of range.
	p.values[i]=valstr                    // Set the value at index i with this format.
	switch{                               // Act according to the value in valstr.
	  case strings.ContainsRune(valstr,'"'): // Is it a double quote?
		  p.quotes[i]='\''                  // Yes, set the quote to single quote.
		case strings.ContainsAny(valstr,"',"):// Is it a single quote or comma?
		  p.quotes[i]='"'                   // Yes, set the quote to double quote.
		default:                            // Else it is neither.
		  p.quotes[i]=0                     // So set the quote to nothing.
	}                                     // Done acting according to the value.
	p.n=uint(len(p.values))               // We now have this many values.
	return nil														// We are good if we got here.
}                                       // --------- SetValueInFormat ------- //

func (s *Section) MakeShallowCopyOf(src *Section){
  s.parentNames=src.parentNames         // The names of the parents if any.
	s.parents=src.parents                 // Array of parent sections if any.
  s.nParents=src.nParents               // Number of parents of this section.
	s.nParameters=src.nParameters         // Number of Parameter objects.
	s.nSections=src.nSections             // Number of Section objects.
	s.firstSection=src.firstSection       // The linked list of sections
	s.lastSection=src.lastSection         //  referenced in this section.
	s.first=src.first                     // The linked list of Parameters in
	s.last=src.last                       //  this section.
	s.current=src.current                 // The currently selected Parameter.
	s.comments=src.comments               // The comments for this section.
	s.copy=true                           // Set the copy flag.
}                                       // --------- MakeShallowCopy -------- //
// =========================== // Configuration // ========================== //
// A class to store the entire configuration file.                            //
// ========================================================================== //
// ------------------------ // NewConfiguration // -------------------------- //
// Initialize the Configuration, Section, and Parameter data structures.      //
// -------------------------------------------------------------------------- //
func NewConfiguration(ext string) (cfg *Configuration){
  cfg.SetDefaultExtension(ext)          // Set the default extension.
	cfg.initialize()                      // Initialize the configuration.
	return cfg                            // Return the configuration object.
}                                       // -------- NewConfiguration -------- //
// ---------------------------- // Initialize // --------------------------- //
// Initialize the configuration data structures. Noop it seems.
// -------------------------------------------------------------------------- //
func (cfg *Configuration) initialize(){
 // Go does this for us automatically.
}
// ---------------------------- // Reconfigure // --------------------------- //
// Prepare to re-read the file by deleting the configuration data structures. //
// Must be called at the beginning of any derived Reconfigure() methods.      //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) Reconfigure() error{
  cfg.deleteAll()                         // Delete all data structures.
	cfg.initialize()                        // Initialize the configuration.
	return nil                            // Always successful.
}
// ----------------------------- // DeleteAll // ---------------------------- //
// Delete the configuration data structures.                                  //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) deleteAll(){
  for p:=cfg.first;p!=nil;p=p.GetNext(){// For each Section in our list...
	  cfg.first,cfg.last,cfg.current=nil,nil,nil// Clear the list.
	}                                     // Done clearing section list.
	for p:=cfg.firstComment;p!=nil;p=p.GetNext(){// For each comment in our list...
	  cfg.firstComment,cfg.lastComment=nil,nil // Clear the list.
	}                                     // Done clearing comment list.
}                                       // ----------- deleteAll ------------ //

// Helpers:
func (cfg *Configuration) SetFilename(fname string) { cfg.path=fname }
func (cfg *Configuration) SetDirectory(dir string) { cfg.path=filepath.Join(dir,filepath.Base(cfg.path))}
func (cfg *Configuration) SetDefaultExtension(ext string) { cfg.ext=ext}
func (cfg *Configuration) GetPathname() string { return cfg.path }
func (cfg *Configuration) GetImportedPathname() string { return cfg.importpath } 
func (cfg *Configuration) GetDirectory() string { return filepath.Dir(cfg.path) }
func (cfg *Configuration) GetFilename() string { return filepath.Base(cfg.path) }
func (cfg *Configuration) GetFirstSection() *Section { return cfg.first }
func (cfg *Configuration) GetLastSection() *Section { return cfg.last }
func (cfg *Configuration) GetFirst() *Section { return cfg.first }
func (cfg *Configuration) GetLast() *Section { return cfg.last }
func (cfg *Configuration) GetSelectedSection() *Section { return cfg.current }
func (cfg *Configuration) GetSectionName() string{
  if cfg.current==nil{ return "" } else { return cfg.current.GetName() }
}
func (cfg *Configuration) GetSection(section string) *Section{
  if cfg.current==nil { return nil } else { return cfg.current.FindSection(section) }
}
// ---------------------- // GetSelectedSectionName // ---------------------- //
// Return a string with the name of the currently selected section.           //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) GetSelectedSectionName() string{
  if cfg.current!=nil{                  // Is a section selected?
	  return cfg.current.GetName()        // Yes, return its name.
	}                                     // Done checking for selected section.
	return ""                             // No, return empty string.
}                                       // ------ GetSelectedSectionName ---- //
// ------------------- // GetSelectedSectionParentName // ------------------- //
//  Return a pointer to the name of the parent of the currently selected      //
// section.                                                                   //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) GetSelectedSectionParentName() string{
  if cfg.current!=nil{                  // Is a section selected?
	  return cfg.current.GetParentName(0) // Yes, return its parent's name.
	}                                     // Done checking for selected section.
	return ""                             // No, return empty string.
}                                       // -- GetSelectedSectionParentName -- //
// ------------------------ // GetFirstSectionName // ----------------------- //
// Return a string with the name of the first section in the list.            //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) GetFirstSectionName() string{
  if cfg.first!=nil{                    // Are there any sections?
	  return cfg.first.GetName()          // Yes, return the first section name.
	}                                     // Done checking for first section.
	return ""                             // No, return empty string.
}                                       // ------ GetFirstSectionName ------- //
// --------------------------- // SaveComments // --------------------------- //
// Set or clear the flag that says we are saving comments.                    //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) SaveComments(flag bool){
  cfg.saveComments=flag                 // Save comments if true.
}                                       // ----------- SaveComments --------- //
func (cfg *Configuration) IgnoreImports(flag bool){
  cfg.ignoreImports=flag                // Ignore imports if true.
}                                       // ----------- IgnoreImports -------- //
// ------------------------------ // ReadFile // ---------------------------- //
// Read a configuration file into the internal data structures.               //
//                                                                            //
// Notes:                                                                     //
//  1. Multiple new statements for Comment objects using **c and              //
//     *firstComment. This method builds a linked list of comments as it      //
//     finds them. It remembers the head of the list in *firstComment, and    //
//     used **c to store the tail of the list. When a Section or Parameter is //
//     found, the head of the list of Comments is attached to that Section or //
//     Parameter. If the end of file is reached without finding a Section or  //
//     Parameter for the Comments, the list is attached to the Configuration  //
//     object. After the list is attached to either a Section or Parameter    //
//     object, the variables **c and *firstComment are re-used for the next   //
//     list of comments.                                                      //
//  2. Imported sections cannot have parent sections in the file being        //
//     imported. You can only have one level of inheritance.                  //
// ------------------------------------ //                                    //
func (cfg *Configuration) ReadFile(     // Nil error if successful.           //
  filename string,                      // Filename or pathname to read.      // 
	section string,                       // Section to read if importing       //
	importing bool)error{                 // True if importing.                 //
                                        // ------------ ReadFile ------------ //
  f,err:=os.Open(filename)              // Open the file for reading. 
  if err!=nil{                          // Error opening the file?
    return fmt.Errorf("error opening file %s: %w", filename, err)// Yes, return error.
  }                                     // Done checking for error opening file.
	defer f.Close()                       // Close the file when done.
	cfg.path=filename                     // Store the last opened file path.
	const linelen=32*1024                 // Maximum line length is 32KiB.
	reader:=bufio.NewReaderSize(f,linelen)// Buffered reader to read the file.
	var(                                  // Our local variables list to hold state info.
	  lineno     int                      // The current line number.
		inBlock bool                        // True if we are inside a block comment.
		searching=section!=""               // True if we are searching for a section.
		importingSect=false                 // True if we are importing a section.
		cHead     *Comment                  // The head of the comment list.
		cTail     *Comment                  // The tail of the comment list.
		currSect  *Section                  // The current section we are reading.
	)                                     // Done declaring state variables.
	// ---------------------------------- //
	// Ad-hoc function to handle tail and head of the comment list and flush them
	// to the target object.
	// ---------------------------------- //
	flushComments:=func(target interface{}){// Append the comments to the target object.
	  if cHead==nil{                      // Any comments in the list to flush?
		  return                            // No comments to flush.
		}                                   // Done checking for comments.
		switch t:=target.(type){            // Switch on the target type.
		  case *Configuration:              // Are we dealing with a Configuration?
		    if cfg.firstComment==nil{       // Any comments yet?
				  cfg.firstComment=cHead        // No, so this is our first comment in list.
				} else{                         // Else we have comments in the list.
				  cfg.lastComment.SetNext(cHead)// Append the new comments to the end of the list.
				}                               // Done checking for first comment.
				cfg.lastComment=cTail           // Now we have a new last comment.
			case *Section:                    // Are we dealing with a Section?
		    t.comments=cHead                // Yes, so set the comments for the section.
			case *Parameter:                  // Are we dealing with a Parameter?
			  t.comments=cHead                // Yes, so set the comments for the parameter.	  
		}                                   // Done acting according to target object type.
		cHead,cTail=nil,nil                 // Clear the comment list for next time.                
	}                                     // Done defining the flushComments function.
	// ---------------------------------- //
	// Ad-hoc function to append a new Comment object to the list and flush it
	// to the target object.
	// ---------------------------------- //
	appendComment:=func(raw string){      // Append a new comment to the list.
	  c:=NewComment(raw,importing)        // Create a new Comment object.
		if c==nil{                          // Could we create a new Comment?
		  return                            // No, so just return.
		}                                   // Done creating a new Comment.
		if cHead==nil{                      // Any comments in the list?
		  cHead=c                           // No, make this one the fist of the list.
		} else{                             // Else we have comments in the list.
		  cTail.SetNext(c)                  // So make this on the next one.
		}                                   // But now we have a new tail.
		cTail=c                             // Set the tail to the new comment.
	}                                     // Done defining the appendComment function.
	// ---------------------------------- //
	// Now we will begin processing the file line by line.
	// ---------------------------------- //
	for{                                  // While we have a sequence of bytes to read...
	  n,err:=reader.ReadBytes('\n')       // Read a line from the file.
		eof:=errors.Is(err,io.EOF)          // Is it the end of the file?
		if err!=nil&&!eof{                  // An error and not EOF?  
		  return fmt.Errorf("error reading file %s at line %d: %w", filename, lineno, err)// Yes, return error.
		}                                   // Done checking for error reading file.
		lineno++                            // Increment the line number.
		n=bytes.TrimRight(n,"\r\n")         // Remove trailing newlines/carriage returns.
		// Handle block comments (comments that start with /* and end with */).
		if bytes.HasPrefix(n,[]byte("/*")){ // Are we entering a block comment?
		  inBlock=true                      // Yes, so set the flag.
		}                                   // Done checking for block comment start.
		if inBlock{                         // Are we inside a block comment?
		  appendComment(string(n))          // Yes, so append the comment to the list.
			if bytes.HasSuffix(n,[]byte("*/")){// Are we leaving the comment block?
			  inBlock=false                   // Yes, so clear the flag.
			}                                 // Done checking for block comment end.
			if eof{                           // Are we at the end of file?
			  break                           // Yes, so break out of the loop.
			}                                 // Done checking for end of file.
			continue                          // Skip the rest of the line.
		}                                   // Done checking for block comment.
		// -------------------------------- //
		// We are done checking for block comments. Now we have to check for
		// continuation lines, by checking if the line ends with a backslash.
		// -------------------------------- //
		for bytes.HasSuffix(n,[]byte{'\\'})&&!eof{// While we have a continuation line...
		  n=n[:len(n)-1]                    // Remove backslash from end of the line.
			next,_:=reader.ReadBytes('\n')    // Read the next line from the file.
			next=bytes.TrimLeft(next," \t")   // Remove leading whitespace from the next line.
			n=append(n,next...)               // Append the next line to the current line.
			lineno++                          // Increment the line number.
		}                                   // Done checking for continuation lines.
		// -------------------------------- //
		// Now we are done checking for block comments and continuation lines.
		// We will now check if the line is empty or not, and append the comment
		// to the list.
		// -------------------------------- //
		line:=strings.TrimSpace(string(n))  // Trim whitespace from the line.
		if line==""{                        // Is the line empty?
		  appendComment(string(n))          // Yes, but we are in a comment block, so append it.
		  if eof{                           // No more bytes to process?
			  break                           // Yes, break out of the loop.
			}                                 // Done checking for empty line.
			continue                          // Skip the rest of the line.
		}                                   // Done checking for empty line.
		// -------------------------------- //
		// We now proceed to classify the line and process it accordingly.
		// -------------------------------- //
		switch{                             // Act according to the line content.
		  // Comments
			case strings.HasPrefix(line,"#"): // Is it a comment line?
			  appendComment(line)             // Yes, so append it to the comment list.
			// Read "file.cfg"
			case strings.HasPrefix(line,"read \""):// Is it a read statement?
			  flushComments(cfg)              // Yes, flush comment to Configuration object.
				fname:=strings.TrimSpace(line[5:])// Get the filename from the line.
				if len(fname)<2||fname[0]!='"'||fname[len(fname)-1]!='"'{
				  return fmt.Errorf("invalid read statement at line %d: %s", lineno, line)// No, return error.
				}                               // Done checking for malformed read statement.
				target:=fname[1:len(fname)-1]   // Remove the quotes from the filename.
				if err:=cfg.ReadFile(target,"",false);err!=nil{
				  return fmt.Errorf("error reading file %s at line %d: %w", target, lineno, err)// No, return error.
				}                               // Done reading the file.
			// Import "file.cfg"
			case strings.HasPrefix(line,"import "):// Are we importing a whole file?!
				if cfg.ignoreImports||importing{// Are we are already importing?
				  break                         // Yes, so skip, we are already importing.
				}                               // Done checking if importing.
			  flushComments(cfg)              // Yes, flush comment to Configuration object.
				fname:=strings.TrimSpace(line[len("import "):])// Get the filename from the line.
				if len(fname)<2||fname[0]!='"'||fname[len(fname)-1]!='"'{
				  return fmt.Errorf("invalid import statement at line %d: %s", lineno, line)// No, return error.
				}                               // Done checking for malformed import statement.
				target:=fname[1:len(fname)-1]   // Remove the quotes from the filename.
				if err:=cfg.ReadFile(target,"",true);err!=nil{// Read the imported file.
				  return fmt.Errorf("error reading imported file %s at line %d: %w", target, lineno, err)// No, return error.
				}                               // Done reading the imported file.
			// Section Headers
			case line[0]=='[':                // Is it a section header?
			  flushComments(cfg)              // Yes flush section commments to Configuration object.
				if importingSect{               // Are we importing a section?
				  return nil                    // We are done with the imported section.
				}                               // Done checking if importing section.
				sectName,parents,fromfile,err:=cfg.detectSectionHeader((line))// Detect the section header.
				if err!=nil{                    // Could we detect the section header?
				  appendComment(line)           // No, so treat the section hdr as a comment.
					break                         // Skip the rest of the line.
				}                               // Done detecting section header.
				if section!=""&&sectName!=section{// Are we looking for a specific section?
				  break                         // Break, we are still looking for it.
				}                               // Done checking for section name.
				searching=false                 // We are no longer searching for a section.
				currSect=cfg.AppendSection(sectName,cHead,importing)// Append a new Section object.
				currSect.SetParentNames(parents)// Set the parent names for the section.
				flushComments(currSect)         // Flush the comments to the section.
				if fromfile!=""{                // Is there a file to import from?
				  if err:=cfg.ReadFile(fromfile,sectName,true);err!=nil{// Read from imported file.
					  return fmt.Errorf("error reading imported file %s at line %d: %w", fromfile, lineno, err)// No, return error.
					}                             // Done reading imported file.
				}                               // Done checking for imported file.
			// Parameters
			default:                          // Any other case to handle?
			  if currSect==nil||searching{    // Are we searching for a section?
				  appendComment(string(n))      // Yes, so treat the line as a comment.
          break                         // Skip the rest of the line.
				}                               // Done checking for searching section.
				name,values,err:=cfg.detectParameter(line)// Detect the parameter.
				if err!=nil{                    // Could we detect the parameter?
				  appendComment(string(n))      // No, so treat the line as a comment.
					break                         // Skip the rest of the line.
				}                               // Done detecting parameter.
				// ---------------------------- //
				// If the line is of the form Ref=[SectionName], we don't want a 
				// parameter called Ref, but rather a shallow-copy of [sectionName].
				// So we check for opening and closing brackets of the value of Ref and
				// build the copy.
				// ---------------------------- //
				if len(values.arr)==1{          // Do we have a single value?
				  v:=strings.TrimSpace(values.arr[0])// Yes, get the value.
					if strings.HasPrefix(v,"[")&&strings.HasSuffix(v,"]"){// Is it a section reference?
					  target:=strings.TrimSpace(v[1:len(v)-1])// Yes, get the section name.
						ref:=cfg.AppendSection(name,cHead,importing)// make [Ref]
						flushComments(ref)          // Flush the comments to the section.
						if tgt:=cfg.FindSection(target);tgt!=nil{// Did we find the section?
				  // -------------------------- //
					// If we have already read the section, we can make a shallow copy
					// of it. Otherwise, the section is not yet read, so we will
					// save its name and reserve space for it so that we can
					// fill it in later using resolveSectionRefs().
					// -------------------------- //
							ref.MakeShallowCopyOf(tgt)// Make a shallow copy of the section.
						} else{                     // Else the section is later in the file.
						  ref.parentNames=[]string{target}// Save the section name.
							ref.nParents=1            // We have one parent section.
							ref.parents=make([]*Section,1)// Reserve space for the parent section.
						}                           // Done checking for section reference.
						break                       // Skip the rest of the line.
				  }                             // Done checking for section reference.
				}																// Done checking for single value.
				p:=currSect.AppendParameter(name,values.raw,cHead,importing)// Append a new Parameter object.
				flushComments(p)                // Flush the comments to the parameter.
		}                                   // Done acting according to the line content.
		if eof{                             // Are we at the end of the file?
		  break                             // Yes, so break out of the loop.
		}                                   // Done checking for end of file.
	}                                     // Done iterating through the file.
	flushComments(cfg)                    // Flush any remaining comments to the Configuration object.
	cfg.resolveParents()                  // Resolve the parent sections for all sections.
	cfg.resolveSectionRefs()              // Resolve the section references for all sections.
	return nil                            // Return nil error if successful.
}                                       // ------------ ReadFile ------------ //
// ----------------------------- // SplitCSVList // ------------------------- //
// Split a comma-separated list of values into a slice of strings.            //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) splitCSVList(list string) []string{
  var(                                  // Local variables for the function.
	  res  []string                       // The result slice of strings.
		curr strings.Builder						    // A string builder to build the values.
		inquotes bool                       // Are we inside quotes?
		quote rune	                        // Where to store a quote character if any.
	)                                     // Done declaring local variables.
	for _,v:=range list{                  // For each character in the list....
	  switch {                            // Act according to the character.
		  case (v=='\''||v=='"')&&(!inquotes||v==quote):
			  if inquotes && v==quote{        // Are we inside quotes and found another quote?
				  inquotes=false                // Yes, then that must be the closing one.
				} else if !inquotes{            // Else we are about to enter a quote
				  inquotes,quote=true,v         // So say that.
				}                               // Done checking for quotes.
			case v==','&&!inquotes:           // Is it a comma and not inside quotes?
			  res=append(res,strings.TrimSpace(curr.String()))// Yes, add the current value to the result.
		    curr.Reset()                    // Reset the string builder.
			default:                          // Any other character.
			  curr.WriteRune(v)               // Just add it to the current value.
		}                                   // Done acting according to these characters.
	}                                     // Done iterating through the list.
	res=append(res,strings.TrimSpace(curr.String()))// Add the last value to the result.
	return res                            // Return the result slice of strings.
}                                       // ----------- SplitCSVList --------- //

// ----------------------- // detectSectionHeader // ------------------------ //
// Detect if the current line is a section header, and if it is... Then, we   //
// check if it has parents, or if it is an imported section.                  //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) detectSectionHeader(line string) (name,parents,fromfile string,err error){
  if !strings.HasPrefix(line,"["){      // Is it a section header?
	  return "","","",fmt.Errorf("line \"%s\" is not a section header",line)// No
	}                                     // Done checking if not section header.
	end:=strings.IndexByte(line,']')      // Find the end of the section header.
	if end==-1{                           // No Closing bracket?
	  return "","","",fmt.Errorf("line \"%s\" is not a valid section header",line)// No
	}                                     // Done checking for closing bracket.
	field:=strings.TrimSpace(line[1:end]) // Get the field name.
	if colon:=strings.IndexByte(field,':');colon!=-1{// Is there a colon in the field?
	  name=strings.TrimSpace(field[:colon])// Yes get string up to the colon.
		parents=strings.TrimSpace(field[colon+1:])// Get the parents after the colon.
	} else{                               // Else there is no colon...
	  name=field                          // So there is no hierarchical relationship.
	}                                     // Done checking for colon.
	rest:=strings.TrimSpace(line[end+1:]) // Get the rest of the line after the closing bracket.
	if rest==""{                          // Nothing after the closing bracket.
	  return name,parents,"",nil          // Return the name and parents. 
	}                                     // Done checking if anything after ']' bracket.
	const keyword string="inherits "      // The keyword to look for.
	if !strings.HasPrefix(rest,keyword){  // Does it start with the keyword?
	  return "","","",fmt.Errorf("line \"%s\" is not a valid import statement",line)// No
	}                                     // Done checking for keyword.
	rest=strings.TrimSpace(rest[len(keyword):])// Remove the keyword.
	if len(rest)<2||rest[0]!='"'||rest[len(rest)-1]!='"'{// Is it a quoted string?
	  return "","","",fmt.Errorf("inherit statement in line \"%s\" must quote filename",line)
	}                                     // No, return error.
	fromfile=rest[1:len(rest)-1]          // Remove quotes to get filename.
	return name,parents,fromfile,nil      // Return the name, parents, and filename.
}                                       // ----- detectSectionHeader -------- //
// -------------------------- // detectParameter // ------------------------- //
// Detect if the current line is a parameter, and if it is... Then, we check  //
// if it has a value, or if it is a reference to another section.             //
// -------------------------------------------------------------------------- //
type paramVals struct{
  raw string														// The raw value of the parameter.
	arr []string                          // The array of values for the parameter.
}
func (cfg *Configuration) detectParameter(line string) (name string,vals paramVals, err error){
  eq:=strings.IndexByte(line,'=')       // Find the equals sign in the line.
	if eq<=0{                             // Do we have an equals sign?
	  return "",vals,fmt.Errorf("line \"%s\" is not a valid parameter",line)// No, return error.
	}                                     // Done checking for equals sign.
	name=strings.TrimSpace(line[:eq])     // Get everything before the equals sign.
  vals.raw=strings.TrimSpace(line[eq+1:])// Get everything after the equals sign.
	vals.arr=cfg.splitCSVList(vals.raw)   // Split the values by commas.
	return name,vals,nil                  // Return the name and values.
}                                       // -------- detectParameter --------- //
// -------------------------- // resolveParents // -------------------------- //
// Resolve parent sections for all sections in the configuration.
// -------------------------------------------------------------------------- //
func (cfg *Configuration) resolveParents(){
  for s:=cfg.first;s!=nil;s=s.GetNext(){// For each section in the configuration...
	  i:=uint(0)                          // Start with the first parent.
		for i<s.GetNParents(){              // For the number of parents...
		  name:=s.GetParentName(i)          // Get THIS parent name.
			parent:=cfg.FindSection(name)     // Find the parent section by name.
			if parent!=nil{                   // Did we find the parent section?
			  s.SetParentSection(i,parent)    // Yes, then set the parent section.
				i++                             // Increment the parent index.
			} else{                           // Else we could not find the parent section.
			  s.RemoveMissingParent(i)        // So remove the missing parent.
			}                                 // Done checking if we found the parent section.
		}                                   // Done iterating through parents.
	}                                     // Done iterating through sections.
}                                       // ---------- resolveParents -------- //
// ------------------------ // resolveSectionRefs // ------------------------ //
// Resolve section references for all sections in the configuration.
// -------------------------------------------------------------------------- //
func (cfg *Configuration) resolveSectionRefs(){
  for s:=cfg.first;s!=nil;s=s.GetNext(){// For each section in the configuration...
	  for ref:=s.firstSection;ref!=nil;ref=ref.GetNext(){// For each section reference...
		  target:=cfg.FindSection(ref.GetName())// Find the target section.
			if target!=nil{                   // Did we find the target section?
			  ref.MakeShallowCopyOf(target)   // Yes, so make a shallow copy of it.
			}                                 // Done checking if we found the target section.
		}                                   // Done iterating through section references.
	}                                     // Done iterating through sections.
}                                       // ------- resolveSectionRefs ------- //
// --------------------------- // Print // ---------------------------------- //
// Print the configuration to a buffered writer.
// -------------------------------------------------------------------------- //
func (cfg *Configuration) Print(w io.Writer) (int64,error){
  var n int64                           // The number of bytes written.
	// ---------------------------------- //
	// Print the file-level comments to the buffered writer.
	// ---------------------------------- //
	for c:=cfg.firstComment;c!=nil;c=c.GetNext(){// For each comment in the list...
	  if !c.IsImported()||c.IsImportStatement(){// Is it an import statement?
		  if _,err:=w.Write([]byte(c.value+"\n"));err!=nil{// Try to write the comment.
			  return n,err                    // Return error if failed to write.
			}                                 // Done writing comment.
		}                                   // Done checking if comment is import statement.
	}                                     // Done iterating through comments.
	// ---------------------------------- //
	// Write the sections in order.
	// ---------------------------------- //
	for s:=cfg.first;s!=nil;s=s.GetNext(){// Starting from the first section...
	  m,err:=s.Print(w)                   // Print the section to the buffered writer.
		n+=m                                // Add the number of bytes written.
		if err!=nil{                        // Error printing the section?
		  return n,err                      // Yes, return error.
		}                                   // Done checking for error printing section.
	}                                     // Done iterating through sections.
	return n,nil                          // Return # of bytes written and nil error.
}                                       // ----------- Print ---------------- //
// ------------------------------ // NewFile // ----------------------------- //
// Allow writing a new file, and optionally give filename.                    //
// Note: If the comments from the original file were not saved, the new file  //
//       will not have any comments.                                          //
// ________type/name___________ _________________description_________________ //
// string fileName              File to write to.                             //
func (cfg *Configuration) NewFile(filename string) error{
  cfg.SetFilename(filename)             // Set the filename to write to.
	cfg.canWrite=true                     // Set the flag that we can write to the file.
	return nil                            // Always successful, return nil error.
}                                       // ------------- NewFile ------------ //
// ----------------------------- // WriteFile // ---------------------------- //
// Write a configuration file from the internal data structures.              //
// ________type/name___________ _________________description_________________ //
// string fileName              File to write to.                             //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) WriteFile(filename string) error{
  if !cfg.canWrite{                     // Can we write to the file?
	  return fmt.Errorf("configuration is not writable")// No, return error.
	}                                     // Done checking if we can write.
	if filename!=""{                      // Did they give us a filename?
	  cfg.SetFilename(filename)           // Yes, so set the filename.
	} else if cfg.GetPathname()==""{      // We have no pathname stored and no filename given?
	  return fmt.Errorf("no filename given and no pathname set")// No, return error.
	}                                     // Done checking for filename.
	f,err:=os.Create(cfg.GetPathname())   // Create the file to write to.
	if err!=nil{                          // Error creating the file?
	  return err                          // Yes, return error.
	}                                     // Done checking for error creating file.
	defer f.Close()                       // Close the file when done.
	buf:=bufio.NewWriter(f)               // Our buffered writer.
	if _,err:=cfg.Print(buf);err!=nil{    // Try to write the configuration to the file.
	  return err                          // Return error if any.
	}                            // Done checking for error writing configuration.
  return buf.Flush()                    // Flush the buffered writer to the file.
}                                       // ----------- WriteFile ------------ //
// -------------------------- // AppendSection // --------------------------- //
// Create a new Section and select it as the default section.                 //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) AppendSection(name string,comments *Comment,importing bool) *Section{
  p:=NewSection(cfg,name,comments,importing)// Make a new Section.
	if cfg.first==nil{                    // Is our section list empty?
	  cfg.first=p                         // Yes, so make this the first section.
	} else{                               // Else we already have sections.
	  cfg.last.SetNext(p)                 // So append this section to the end of the list.
	}                                     // Done appending section to the list.
	cfg.last=p                            // Now we have a new last section.
	return p                              // Return the new Section object.
}                                       // ----------- AppendSection -------- //
// ------------------------- // FindSection // ------------------------------ //
// Look for a Section by name.                                                //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) FindSection(name string) *Section{
  for s:=cfg.first;s!=nil;s=s.GetNext(){// For each section in the configuration...
	  if strings.EqualFold(s.GetName(),name){// Is it the section we are looking for?
		  return s                          // Yes, we found the section, return it.
		}                                   // Done checking for section.
	}                                     // Done iterating through sections.
	return nil                            // No match found, return nil.
}                                       // ------------ FindSection --------- //
// ------------------------- // SelectSection // ---------------------------- //
// Set default section for Get & Set Parameter calls without section names.   //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) SelectSection(name string) error{
  cfg.current=cfg.FindSection(name)     // Find the section by name.
	if cfg.current!=nil{                  // Did we find the section?
	  cfg.current.SelectFirstParameter()  // Yes, select first parameter in section.
		return nil                          // Return nil error if successful.
	}                                     // Done checking if we found the section.
	return fmt.Errorf("section \"%s\" not found", name) // No, return error.
}                                       // ----------- SelectSection -------- //
// ---------------------- // ClearParameters // ----------------------------- //
// Erase all parameters in the current section.
// -------------------------------------------------------------------------- //
func (cfg *Configuration) ClearParameters(section string) error{
  if cfg.first!=nil && cfg.SelectSection(section)==nil{// Could we select the section?
	  cfg.current.ClearParameters()       // Yes, so clear the parameters in the section.
		return nil                          // Return nil error if successful.
	}                                     // Done checking if we could select the section.
	return fmt.Errorf("section \"%s\" not found", section) // No, return error.
}                                       // --------- ClearParameters -------- //
// -------------------- // GetNextParameterValues // ------------------------ //
// Get next parameter from the default section.
// If values=nil, the user is just getting the name of the next parameter.
// -------------------------------------------------------------------------- //
func (cfg *Configuration) GetNextParameterValues(vals [][]string,q []string) (name []string, nValues int, values [][]string,quotes []string,err error){
  s:=cfg.current                        // Get the current section.
	if s!=nil{                            // Is there a current section?
	  p:=s.GetSelectedParameter()         // Get the selected parameter.
		if p!=nil{                          // Did we find a parameter?
		  name[0]=p.GetName()               // Yes, get it's name.
			nValues=int(p.GetNValues())       // Get the number of values.
			if vals!=nil{                     // Did they give us any values?
			  values[0]=p.GetValueArray()     // Yes, so get the values.
			}                                 // Done checking for values.
			if q!=nil{                        // Did they give us any quotes?
			  quotes[0]=string(p.quotes)      // Yes, so get the quotes.
			}                                 // Done checking for quotes.
			s.SelectParameter(p.GetNext())    // Select the next parameter in the section.
      return name,nValues,values,quotes,nil// Return what we found.
		} else{                             // Else no more in this section.
		  return nil,0,nil,nil,fmt.Errorf("no more parameters in section \"%s\"", s.GetName())// No, return error.
		}                                   // Done checking for parameter.
	}                                     // Done checking for current section.
	return nil,0,nil,nil,fmt.Errorf("no current section selected") // No current section, return error.
}                                       // ----- GetNextParameterValues ----- //
// -------------------- // GetNextParameterValues // ------------------------ //
// Get next parameter from the default section.
// If values=nil, the user is just getting the name of the next parameter.
// -------------------------------------------------------------------------- //
func (cfg *Configuration) GetNextParameterValues2(vals [][]string) (name []string, values []string,err error){
  s:=cfg.current                        // Get the current section.
	if s!=nil{                            // Is there a current section?
	  p:=s.GetSelectedParameter()         // Get the selected parameter.
		if p!=nil{                          // Did we find the default parameter?
		  name[0]=p.GetName()               // Yes, get it's name.
			if vals!=nil{                     // Did they give us any values?
			  values[0]=p.GetValue(0)         // Yes, so get the value.
			}                                 // Done checking for values.
			s.SelectParameter(p.GetNext())    // Select the next parameter in the section.
			return name,values,nil            // Return what we found.
		} else{                             // Else no more in this section.
		  return nil,nil,fmt.Errorf("no more parameters in section \"%s\"", s.GetName())// No, return error.
		}                                   // Done checking for parameter.
	}                                     // Done checking for current section.
	return nil,nil,fmt.Errorf("no current section selected") // No current section, return error.
}                                       // ----- GetNextParameterValues2 ---- //
// --------------------- // GetNextParameter // ----------------------------- //
// Get next parameter from the default section.
// -------------------------------------------------------------------------- //
func (cfg *Configuration) GetNextParameter() (*Parameter){
  s:=cfg.current                        // Get the current section.
	if s!=nil{                            // Do we have a section?
	  p:=s.GetSelectedParameter()         // Get the selected parameter.
		if p!=nil{                          // Did we find a parameter?
		  s.SelectParameter(p.GetNext())    // Yes, so select the next parameter.
			return p                          // Return the parameter.
		}                                   // Done checking for parameter.
	}                                     // Done checking for current section.
	return nil                            // No current section, return nil.                    
}                                       // -------- GetNextParameter -------- //
func (cfg *Configuration) SelectParameter(name string) error{
  if cfg.current!=nil{                  // Do we have a current section?
	  return cfg.current.SelectParameterByName(name)// Yes, so select the parameter by name.
	}                                     // Done checking for current section.
	return fmt.Errorf("no current section selected") // No current section, return error.
}                                       // -------- SelectParameter --------- //
// ---------------------------- // GetValue // ------------------------------ //
// Get a parameter value from the currently-selected section.                 //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) GetValue(name string) string{
  if cfg.current!=nil{                  // Do we have a current section?
	  return cfg.current.GetValue(name,uint(0))// Get the value of the parameter.	
	}                                     // Done checking for current section.
	return ""                             // No current section, return empty string.
}                                       // ------------- GetValue ----------- //
func (cfg *Configuration) GetValues(name string) string{
  if cfg.current!=nil{                  // Do we have a current section?
    return cfg.current.GetValues(name)  // Yes, return the value of this parameter.	
	}                                     // Done checking for current section.
	return ""                             // No current section, return empty string.
}                                       // ------------- GetValues ---------- //
// ------------------------- // GetValueByIndex // -------------------------- //
// Get a parameter value from the currently-selected section.                 //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) GetValueByIndex(name string, i uint) string{
  if cfg.current!=nil{                  // Do we have a current section?
	  return cfg.current.GetValue(name,uint(i))// Get the value of the parameter.	
	}                                     // Done checking for current section.
	return ""                             // No current section, return empty string.
}                                       // -------- GetValueByIndex --------- //
// ---------------------------- // GetValue // ------------------------------ //
// Get a parameter from a particular Section without selecting that Section.  //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) GetValueBySection(section, name string) string{
  s:=cfg.FindSection(section)           // Find the section by name.
	if s!=nil{                            // Did we find the section?
	  return s.GetValue(name,uint(0))     // Yes, return the value of this parameter.	
	}                                     // Done checking for current section.
	return ""                             // No current section, return empty string.
}                                       // -------- GetValueByIndex --------- //
// ---------------------------- // GetValue // ------------------------------ //
// Get a parameter from a particular Section without selecting that Section.  //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) GetValueBySectionAndIndex(section, name string, i uint) string{
  s:=cfg.FindSection(section)           // Find the section by name.
	if s!=nil{                            // Did we find the section?
	  return s.GetValue(name,uint(i))     // Yes, return the value of this parameter.	
	}                                     // Done checking for current section.
	return ""                             // No current section, return empty string.
}                                       // --- GetValueBySectionAndIndex ---- //
func (cfg *Configuration) GetValueBool(name string,i uint,tval string, fval string) (result bool, err error){
  
	p:=cfg.GetValueByIndex(name,i)        // Get the parameter value.
	if p!=""{                             // Did we get a value?
	  if tval==""&&fval==""{              // No values given?
		  if isTrue(p){                     // Is it "true"?
			  result=true                     // Yes, set result to true.
			}else if isFalse(p){              // Is it "false"?
			  result=false                    // Yes, set result to false.
			} else{                           // Else we dont know what it is.
			  result=false                    // So make it false.
				err=fmt.Errorf("value %s is not a boolean", p)// Store error.
			}                                 // Done checking if could decode p.
		}else if tval!=""&&fval!=""{        // Caller gave possible values?
		  if strings.Contains(tval,p){      // Is it a true value?
			  result=true                     // Yes set result to true.
			}else if strings.Contains(fval,p){// Is it a false value?
			  result=false                    // Yes set result to false.
			} else{                           // Otherwise we cant decode it.
			  result=false                    // Set result to false.
				err=fmt.Errorf("value %s is not a boolean", p)// Store error.
			}                                 // Done checking if can decode p.
		}                                   // Done checking if caller gave values.
	}                                     // Done checking for value.
  return result,err                     // Return result and error if any.
}                                       // ----------- GetValueBool --------- //
// --------------------------- // GetNValues // ----------------------------- //
// Return the number of values in the given Parameter of the selected Section.//
// -------------------------------------------------------------------------- //
func (cfg *Configuration) GetNValues(name string) uint{
  if cfg.current!=nil{                  // Do we have a current section?
	  return cfg.current.GetNValues(name) // Yes, return the number of values.
	}                                     // Done checking for current section.
	return 0                              // No current section, return 0.
}                                       // ----------- GetNValues ----------- //
// ------------------------- // GetNParameters // --------------------------- //
//  Get the number of parameters in the given section, or the currently-      //
// selected Section. The currently-selected Section will not be changed.      //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) GetNParameters(section string) uint{
  s:=cfg.current                        // Get the current section as default.
	if section!=""{                       // Were we given a section?
	  s=cfg.FindSection(section)          // Yes, so find the section by name.
	}                                     // Done checking for section name.
	if s!=nil{                            // Did we find the section?
	  return s.GetNParameters()           // Yes, return the number of parameters.
	}                                     // Done checking if we found the section.
	return 0                              // No section found, return 0.
}                                       // ---------- GetNParameters -------- //
// ----------------------------- // ScanValue // ---------------------------- //
// Scan the requested value into the destination variable using the specified
// format. The destination variable must the type of value
// being scanned. The format string must match the type of the value being
// scanned. The value is scanned from the string representation of the value
// in the configuration file. The value is converted to the type of the
// destination variable using the format string.
// -------------------------------------------------------------------------- //
func (cfg *Configuration) scanValue(name string,i int, format string, dest any) error{
  p:=cfg.current.FindParameter(name,true)// Find the parameter in this section.
	if p==nil{                            // Did we find the parameter?
	  return fmt.Errorf("parameter %s not found in section %s", name, cfg.current.name)// No, return error.
	}                                     // Done checking for parameter.
	if i < 0 || i >= int(cfg.current.GetNValues(name)){       // Within range?
	return fmt.Errorf("index %d out of range", i)
  }                                     // Done checking for out of range.                                // Done checking for out of range.
  var verb byte                         // The format verb.
  for j:=1;j<len(format);j++{           // For each character in fmt string...
	  c:=format[j]                        // Get the j'th characer.
		// -------------------------------- //
		// We need to check if 'c' is one of the characters that can legally
		// appear in the *flags, width, precision or modifier* part of a fmt string.
		// if it is, we will skip it and continue scanning the next character.
		// -------------------------------- //
		if strings.ContainsRune("#0- +. '0123456789*",rune(c)){ continue }
		// -------------------------------- //
		// Otherwise we have reached the first character that is not a flag, digit,
		// dot or star, so by definition this is the format verb.
		// -------------------------------- //
		verb=c                              // The format verb.
		break                               // We found the verb so break from loop.
	}                                     // Done iterating through the format string.
  if verb!=0&&!verbComaptible(verb,reflect.TypeOf(dest).Elem().Kind()){
	  return fmt.Errorf("format %s is not compatible with type %s", format, reflect.TypeOf(dest).Elem().Kind())
	}                                     // Done checking for format compatibility.
  val:=reflect.ValueOf(dest).Kind()     // Get the type of the dest variable.
	if dest==nil||val!=reflect.Ptr{       // Is dest nil or not a pointer?
	return errors.New("destination must be a non-nil pointer")// Yes, return an error.
  }                                     // Done checking for nil or pointer
	// Scan the value into the destination variable
  _,err:=fmt.Sscanf(string(cfg.GetValueByIndex(name,uint(i))),format,dest)
	return err                            // Return error if any.
}                                       // ------------ ScanValue ----------- //

// ---------------------------- // SetValue // ------------------------------ //
// Set a parameter value in the currently-selected section.                   //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) SetValue(name,valuestr string,quote byte) error{
  if cfg.current!=nil{                  // Do we have a current section?
	  return cfg.current.SetValue(name,valuestr,quote)// Yes, set the value of the parameter.
	}                                     // Done checking for current section.
	return fmt.Errorf("no current section selected") // No current section, return error.
}                                       // ------------- SetValue ----------- //

// ------------------------ // SetValueBySection // ------------------------- //
// Set a value for a parameter in a particular Section without selecting that //
// Section.                                                                   //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) SetValueBySection(section,name string, i uint, value string) error{
  s:=cfg.FindSection(section)           // Find the section by name.
	if s!=nil{                            // Did we find the section?
	  return s.SetValuePtr(name,value,0)  // Yes, set the value of the parameter.
	}                                     // Done checking if found section.
	return fmt.Errorf("section \"%s\" not found", section) // No, return error.   
}                                       // ------- SetValueBySection -------- //
// --------------------------- // SetValueInFormat // ----------------------- //
// Set a parameter value in the currently-selected section, using the format
// specified by the format string.
// -------------------------------------------------------------------------- //
func (cfg *Configuration) SetValueInFormat(name string,val any,format string) error{
  if cfg.current==nil{                  // Do we have a current section?
	  return fmt.Errorf("no current sectioon selected") // No current section, return error.
	}                                     // Done checking for current section.
	return cfg.current.SetValueInFormat(name,0,format,val)// Set the parameter's value.
}                                       // --------- SetValueInFormat ------- //
// ------------------------- // SetArrayValue // ---------------------------- //
// Set a parameter value in the currently-selected section.                   //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) SetArrayValue(name,valuestr string,i uint,quote byte) error{
  if cfg.current!=nil{                  // Do we have a current section?
	  return cfg.current.SetValuePtrOnIndex(name,valuestr,i,quote)// Yes, set the value of the parameter.
	}                                     // Done checking for current section.
	return fmt.Errorf("no current section selected") // No current section, return error.
}                                       // ----------- SetArrayValue -------- //
// ---------------------- // SetArrayValueBySection // ---------------------- //
// Set a value for a parameter in a particular Section without selecting that //
// Section.                                                                   //
// -------------------------------------------------------------------------- //
func (cfg *Configuration) SetArrayValueBySection(section,name string, i uint, value string) error{
  s:=cfg.FindSection(section)           // Find the section by name.
	if s!=nil{                            // Did we find the section?
	  return s.SetValuePtrOnIndex(name,value,i,0) // Yes, set the value of the parameter.
	}                                     // Done checking if found section.
	return fmt.Errorf("section \"%s\" not found", section) // No, return error.
}                                       // ----- SetArrayValueBySection ----- //
// --------------------------- // SetValueInFormat // ----------------------- //
// Set a parameter value in the currently-selected section, using the format
// specified by the format string, and the index of the multi-value parameter.
// -------------------------------------------------------------------------- //
func (cfg *Configuration) SetArrayValueInFormat(name string,idx uint,val any,format string) error{
  if cfg.current!=nil{                  // Do we have a current section?
	  return cfg.current.SetValueInFormat(name,int(idx),format,val)// Set the parameter's value.
	}                                     // Done checking for current section.
  return fmt.Errorf("no current section selected") // No current section, return error.
}                                       // ----- SetArrayValueInFormat ------ //

// ---------------- Byte values (character values) -------------------------- //
func (cfg *Configuration) GetValueByte(name string, dest *byte) error{
  p:=cfg.GetValue(name)
	if len(p)==0{
	  return fmt.Errorf("parameter %s not found", name) 
	}
	return cfg.scanValue(name,0,"%c",dest)
}
func (cfg *Configuration)	SetValueByte(name string, value byte) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,string(value),0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueByteByIndex(name string,i uint,dest *byte) error{
  p:=cfg.GetValueByIndex(name,i)
	if len(p)==0{
	  return fmt.Errorf("parameter %s not found", name) 
	}
	return cfg.scanValue(name,int(i),"%c",dest)
}
func (cfg *Configuration)	SetValueByteByIndex(name string, i uint, value byte) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValuePtrOnIndex(name,string(value),i,0)
	}                                     
	return fmt.Errorf("no current section selected")
}

 // ---------------------- Times and durations ------------------------------ //
func (cfg *Configuration)	GetValueTimespec(name string, dest *unix.Timespec) error{
  p:=cfg.GetValue(name)                 
  if len(p)==0{                         
	  return fmt.Errorf("can't decode empty \"value\" to unix.Timespec")
	}                                     
	t,err:=time.Parse(time.RFC3339,p) 
	if err!=nil{                          
	  return fmt.Errorf("can't decode \"%s\" to unix.Timespec: %v", p, err)
	}                                     
	unix.NsecToTimespec(t.UnixNano()) 	  
	*dest=unix.NsecToTimespec(t.UnixNano()) 
	return nil                            
}
func (cfg *Configuration)	GetValueTimespecByIndex(name string,i uint,dest *unix.Timespec) error{
  p:=cfg.GetValueByIndex(name,i)        
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	t,err:=time.Parse(time.RFC3339,p) 
	if err!=nil{                          
	  return fmt.Errorf("can't decode \"%s\" to unix.Timespec: %v", p, err)
	}                                     
	unix.NsecToTimespec(t.UnixNano()) 	  
	*dest=unix.NsecToTimespec(t.UnixNano()) 
	return nil                            
}
func (cfg *Configuration)	GetValueDuration(name string, dest *time.Duration) error{
  p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	d,err:=time.ParseDuration(p)          
	if err!=nil{                          
	  return fmt.Errorf("can't decode \"%s\" to time.Duration: %v", p, err)
	}                                     
	*dest=d                               
	return nil                            
}
func (cfg *Configuration)	GetValueDurationByIndex(name string,i uint,dest *time.Duration) error{
  p:=cfg.GetValueByIndex(name,i)        
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	d,err:=time.ParseDuration(p)          
	if err!=nil{                          
	  return fmt.Errorf("can't decode \"%s\" to time.Duration: %v", p, err)
	}                                     
	*dest=d                               
	return nil                            
}

// Time since epoch
func (cfg *Configuration)	GetValueTime(name string, dest *time.Time) error{
  p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	t,err:=time.Parse(time.RFC3339,p) 
	if err!=nil{                          
	  return fmt.Errorf("can't decode \"%s\" to time.Time: %v", p, err)
	}                                     
	*dest=t                               
	return nil                            
}
func (cfg *Configuration)	GetValueTimeByIndex(name string, i uint,dest *time.Time) error{
  p:=cfg.GetValueByIndex(name,i)        
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	t,err:=time.Parse(time.RFC3339,p) 
	if err!=nil{                          
	  return fmt.Errorf("can't decode \"%s\" to time.Time: %v", p, err)
	}                                     
	*dest=t                               
	return nil                            
}

// --------------------------- Signed Integers ------------------------------ //
func (cfg *Configuration)	GetValueInt(name string, dest *int) error{
  p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%d",dest)
}
func (cfg *Configuration)	SetValueInt(name string, value int) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,strconv.Itoa(value),0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueIntByIndex(name string,i uint,dest *int) error{
  p:=cfg.GetValueByIndex(name,i)        
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,int(i),"%d",dest)
}
func (cfg *Configuration)	SetValueIntByIndex(name string, i uint, value int) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValuePtrOnIndex(name,strconv.Itoa(value),i,0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueInt8(name string, dest *int8) error{
  p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%d",dest)
}
func (cfg *Configuration)	SetValueInt8(name string, value int8) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,strconv.Itoa(int(value)),0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueInt8ByIndex(name string,i uint,dest *int8) error{
  p:=cfg.GetValueByIndex(name,i)        
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,int(i),"%d",dest)
}
func (cfg *Configuration)	SetValueInt8ByIndex(name string, i uint, value int8) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValuePtrOnIndex(name,strconv.Itoa(int(value)),i,0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueInt16(name string, dest *int16) error{
  p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%d",dest)
}
func (cfg *Configuration)	SetValueInt16(name string, value int16) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,strconv.Itoa(int(value)),0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueInt16ByIndex(name string,i uint,dest *int16) error{
  p:=cfg.GetValueByIndex(name,i)        
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,int(i),"%d",dest)
}
func (cfg *Configuration)	SetValueInt16ByIndex(name string, i uint, value int16) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValuePtrOnIndex(name,strconv.Itoa(int(value)),i,0)
	}
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueInt32(name string, dest *int32) error{
  p:=cfg.GetValue(name)
	if len(p)==0{
	  return fmt.Errorf("parameter %s not found", name) 
	}
	return cfg.scanValue(name,0,"%d",dest)
}
func (cfg *Configuration)	SetValueInt32(name string, value int32) error{
  if cfg.current!=nil{
	  return cfg.current.SetValue(name,strconv.Itoa(int(value)),0)
	}
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueInt32ByIndex(name string,i uint,dest *int32) error{
  p:=cfg.GetValueByIndex(name,i)
	if len(p)==0{
	  return fmt.Errorf("parameter %s not found", name) 
	}
	return cfg.scanValue(name,int(i),"%d",dest)
}
func (cfg *Configuration)	SetValueInt32ByIndex(name string, i uint, value int32) error{
  if cfg.current!=nil{
	  return cfg.current.SetValuePtrOnIndex(name,strconv.Itoa(int(value)),i,0)
	}
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueInt64(name string, dest *int64) error{
  p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%d",dest)
}
func (cfg *Configuration)	SetValueInt64(name string, value int64) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,strconv.FormatInt(value,10),0)
	}
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueInt64ByIndex(name string,i uint,dest *int64) error{
  p:=cfg.GetValueByIndex(name,i)        
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,int(i),"%d",dest)
}
func (cfg *Configuration)	SetValueInt64ByIndex(name string, i uint, value int64) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValuePtrOnIndex(name,strconv.FormatInt(value,10),i,0)
	}                                     
	return fmt.Errorf("no current section selected")
}

// --------------------- Unicode, binary and hex values --------------------- //
func (cfg *Configuration)	GetValueRune(name string, dest *rune) error{
  p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%c",dest)
}
func (cfg *Configuration)	SetValueRune(name string, value rune) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,string(value),0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueRuneByIndex(name string,i uint,dest *rune) error{
  p:=cfg.GetValueByIndex(name,i)        
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,int(i),"%c",dest)
}
func (cfg *Configuration)	SetValueRuneByIndex(name string, i uint, value rune) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValuePtrOnIndex(name,string(value),i,0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueBinary(name string, dest *string) error{
  p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%b",dest)
}
func (cfg *Configuration)	SetValueBinary(name string, value string) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,value,0)
	}
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueHex(name string, dest *string) error{
  p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%x",dest)
}
func (cfg *Configuration)	SetValueHex(name string, value string) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,value,0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueOctal(name string, dest *string) error{
  p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%o",dest)
}
func (cfg *Configuration)	SetValueOctal(name string, value string) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,value,0)
	}                                     
	return fmt.Errorf("no current section selected")
}

// ------------------------- Unsigned integers ------------------------------ //
func (cfg *Configuration)  GetValueUint(name string, dest *uint) error{
  p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%u",dest)
}
func (cfg *Configuration)	SetValueUint(name string, value uint) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,strconv.FormatUint(uint64(value),10),0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueUintByIndex(name string,i uint,dest *uint) error{
  p:=cfg.GetValueByIndex(name,i)        
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,int(i),"%u",dest)
}
func (cfg *Configuration)	SetValueUintByIndex(name string, i uint, value uint) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValuePtrOnIndex(name,strconv.FormatUint(uint64(value),10),i,0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueUint8(name string, dest *uint8) error{
  p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%u",dest)
}
func (cfg *Configuration) SetValueUint8(name string, value uint8) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,strconv.FormatUint(uint64(value),10),0)
	}
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueUint8ByIndex(name string,i uint,dest *uint8) error{
  p:=cfg.GetValueByIndex(name,i)        
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,int(i),"%u",dest)
}
func (cfg *Configuration)	SetValueUint8ByIndex(name string, i uint, value uint8) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValuePtrOnIndex(name,strconv.FormatUint(uint64(value),10),i,0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueUint16(name string, dest *uint16) error{
  p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%u",dest)
}
func (cfg *Configuration)	SetValueUint16(name string, value uint16) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,strconv.FormatUint(uint64(value),10),0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueUint16ByIndex(name string,i uint, dest *uint16) error{
  p:=cfg.GetValueByIndex(name,i)        
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,int(i),"%u",dest)
}
func (cfg *Configuration)	SetValueUint16ByIndex(name string, i uint, value uint16) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValuePtrOnIndex(name,strconv.FormatUint(uint64(value),10),i,0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueUint32(name string, dest *uint32) error{
  p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%u",dest)
}
func (cfg *Configuration)	SetValueUint32(name string, value uint32) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,strconv.FormatUint(uint64(value),10),0)
	}
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueUint32ByIndex(name string,i uint,dest *uint32) error{
  p:=cfg.GetValueByIndex(name,i)        
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,int(i),"%u",dest)
}
func (cfg *Configuration)	SetValueUint32ByIndex(name string, i uint, value uint32) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValuePtrOnIndex(name,strconv.FormatUint(uint64(value),10),i,0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueUint64(name string, dest *uint64) error{
  p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%d",dest)
}
func (cfg *Configuration)	SetValueUint64(name string, value uint64) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,strconv.FormatUint(value,10),0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueUint64ByIndex(name string,i uint,dest *uint64) error{
  p:=cfg.GetValueByIndex(name,i)        
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,int(i),"%d",dest)
}
func (cfg *Configuration)	SetValueUint64ByIndex(name string, i uint, value uint64) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValuePtrOnIndex(name,strconv.FormatUint(value,10),i,0)
	}
	return fmt.Errorf("no current section selected")
}

// ------------------------ Floating point values --------------------------- //
func (cfg *Configuration)	GetValueFloat32(name string, dest *float32) error{
  p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%f",dest)
}
func (cfg *Configuration)	SetValueFloat32(name string, value float32) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,strconv.FormatFloat(float64(value),'f',-1,32),0)
	}
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueFloat32ByIndex(name string,i uint,dest *float32) error{
  p:=cfg.GetValueByIndex(name,i)        
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,int(i),"%f",dest)
}
func (cfg *Configuration)	SetValueFloat32ByIndex(name string, i uint, value float32) error{
  if cfg.current!=nil{                  
	  return cfg.current.SetValuePtrOnIndex(name,strconv.FormatFloat(float64(value),'f',-1,32),i,0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueFloat64(name string,dest *float64) error{
	p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%f",dest)
}
func (cfg *Configuration)	SetValueFloat64(name string, value float64) error{
	if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,strconv.FormatFloat(value,'f',-1,64),0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueFloat64ByIndex(name string,i uint,dest *float64) error{
  p:=cfg.GetValueByIndex(name,i)
	if len(p)==0{
	  return fmt.Errorf("parameter %s not found", name) 
	}
	return cfg.scanValue(name,int(i),"%f",dest)
}
func (cfg *Configuration)	SetValueFloat64ByIndex(name string, i uint, value float64) error{
  if cfg.current!=nil{
	  return cfg.current.SetValuePtrOnIndex(name,strconv.FormatFloat(value,'f',-1,64),i,0)
	}
	return fmt.Errorf("no current section selected")
}

// Floating point values with precision
func (cfg *Configuration)	GetValuePrecisionFloat32(name,precision string,dest *float32) error{
	p:=cfg.GetValue(name)
	if len(p)==0{
	  return fmt.Errorf("parameter %s not found", name)
	}
	return cfg.scanValue(name,0,"%"+precision+"f",dest)
}
func (cfg *Configuration)	SetValuePrecisionFloat32(name,precision string,value float32) error{
	if cfg.current!=nil{
	  return cfg.current.SetValue(name,strconv.FormatFloat(float64(value),'f',-1,32),0)
	}
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValuePrecisionFloat32ByIndex(name string,i uint,precision string,dest *float32) error{
	p:=cfg.GetValueByIndex(name,i)
	if len(p)==0{
	  return fmt.Errorf("parameter %s not found", name)
	}
	return cfg.scanValue(name,int(i),"%"+precision+"f",dest)
}
func (cfg *Configuration)	SetValuePrecisionFloat32ByIndex(name string, i uint, precision string, value float32) error{
	if cfg.current!=nil{
	  return cfg.current.SetValuePtrOnIndex(name,strconv.FormatFloat(float64(value),'f',-1,32),i,0)
	}
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValuePrecisionFloat64(name string,value,precision string,dest *float64) error{
	p:=cfg.GetValue(name)
	if len(p)==0{
	  return fmt.Errorf("parameter %s not found", name)
	}
	return cfg.scanValue(name,0,"%"+precision+"f",dest)
}
func (cfg *Configuration)	SetValuePrecisionFloat64(name string,precision string,value float64) error{
	if cfg.current!=nil{
	  return cfg.current.SetValue(name,strconv.FormatFloat(value,'f',-1,64),0)
	}
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValuePrecisionFloat64ByIndex(name string,i uint,precision string,dest *float64) error{
	p:=cfg.GetValueByIndex(name,i)
	if len(p)==0{
	  return fmt.Errorf("parameter %s not found", name)
	}
	return cfg.scanValue(name,int(i),"%"+precision+"f",dest)
}
func (cfg *Configuration)	SetValuePrecisionFloat64ByIndex(name string, i uint, precision string, value float64) error{
	if cfg.current!=nil{
	  return cfg.current.SetValuePtrOnIndex(name,strconv.FormatFloat(value,'f',-1,64),i,0)
	}
	return fmt.Errorf("no current section selected")
}

// ----------------------------- Complex numbers ---------------------------- //
func (cfg *Configuration)	GetValueComplex64(name string,dest *complex64) error{
	p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%v",dest)
}
func (cfg *Configuration)	SetValueComplex64(name string, value complex64) error{
	if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,strconv.FormatComplex(complex128(value),'v',-1,64),0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueComplex64ByIndex(name string,i uint,dest *complex64) error{
	p:=cfg.GetValueByIndex(name,i)        
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,int(i),"%v",dest)
}
func (cfg *Configuration)	SetValueComplex64ByIndex(name string, i uint, value complex64) error{
	if cfg.current!=nil{                  
	  return cfg.current.SetValuePtrOnIndex(name,strconv.FormatComplex(complex128(value),'v',-1,64),i,0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueComplex128(name string,dest *complex128) error{
	p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%v",dest)
}
func (cfg *Configuration)	SetValueComplex128(name string, value complex128) error{
	if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,strconv.FormatComplex(value,'v',-1,64),0)
	}                                     
	return fmt.Errorf("no current section selected")
}
func (cfg *Configuration)	GetValueComplex128ByIndex(name string,i uint,dest *complex128) error{
	p:=cfg.GetValueByIndex(name,i)        
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,int(i),"%v",dest)
}
func (cfg *Configuration)	SetValueComplex128ByIndex(name string, i uint, value complex128) error{
	if cfg.current!=nil{                  
	  return cfg.current.SetValuePtrOnIndex(name,strconv.FormatComplex(value,'v',-1,64),i,0)
	}
	return fmt.Errorf("no current section selected")
}

	// Scientific notation
func (cfg *Configuration)	GetValueSI(name string,dest *string) error{
	p:=cfg.GetValue(name)                 
	if len(p)==0{                         
	  return fmt.Errorf("parameter %s not found", name) 
	}                                     
	return cfg.scanValue(name,0,"%s",dest)
}
func (cfg *Configuration)	SetValueSI(name string, value string) error{
	if cfg.current!=nil{                  
	  return cfg.current.SetValue(name,value,0)
	}                                     
	return fmt.Errorf("no current section selected")
}
