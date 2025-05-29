// **************************************************************************
// Filename:
//  configuration.go
// 
// Description:
//  Interface of the configuration package.
//
// Author:
//  MJB  Matthew J. Bieneman
// 
// Translated by:
//  J.EP  J. Enrique Peraza
//
// ***************************************************************************
package configuration
import (
		"io"
	  "github.com/perazaharmonics/gosys/internal/logger"

)

// =========================== // Comment // ==================================
// A class to store comments and blank lines from a configuration file.
// ============================================================================
type CommentAPI interface{
  NewComment(value string, imported bool) *Comment
	CopyComment(comment *Comment)   *Comment
	IsImportStatement()  bool
	IsImported()         bool
  GetValue()           string
	GetNext()            *Comment
	SetNext(p *Comment)
	Print(w io.Writer)    error
}


type Comment struct{
  imports      bool                     // True if this is an import comment
	isimported   bool                     // True if was imported.
	value       string                     // The text of the comment.
	next        *Comment                  // Where to save next comment on the list.
}

// ========================= // Parameter // ==================================
// A class to store individual parameters and their values.
// ============================================================================
type ParameterAPI interface{
  NewParameter(name, valuestr string,comments *Comment, imported bool) *Parameter
	CopyParameter(*Parameter) *Parameter
	IsImported() bool
	SetValue(valuestr string, quote byte) error
	SetValuePtr(value string,quote byte) error
	SetValuePtrOnIndex(i uint,value string,quote byte) error
	GetNValues() uint
	GetValueArray() []string
	GetValue(i uint) string
  GetValues() string
	GetValueBool(i uint,tval string, fval string) (result bool, err error)
	ScanValue(i int, fmt string, dest any) error
	GetName() string
	GetNext() *Parameter
	SetNext(p *Parameter)
	Append(p *Parameter)
  GetQuote(i uint) (byte,error)
	Print(w io.Writer) (int64,error)
}
type Parameter struct{
  name        string                    // The name of the parameter.
	n           uint                      // The number of values.
	values      []string                  // The values of the parameter.
	quotes      []byte                    // Quote character for each value, 0 if none.
	value       string										// The value of the parameter.
	comments    *Comment                  // The comments associated with this parameter.
	next        *Parameter                // Where to save next parameter on the list.
	isimported   bool                     // True if was imported from another file.
}

// ========================= // Section // =====================================
// A class to store an entire section of a configuration file.
// Note: The comments object is allocated by the caller but deleted by the    //
//       destructor. Eventually we'll change this to make our own copy to     //
//       prevent hierarchy violations.                                        // 
// ========================================================================== //
type SectionAPI interface{
  NewSection(cfg *Configuration, name string, comments *Comment, imported bool) *Section
	NewSection2(cfg *Configuration, name string, prev *Section, comments *Comment, imported bool) *Section
  GetPathname()  string
	GetDirectory() string
	GetFilename()  string
	GetNValues(name string) uint
	GetNParameters() uint
	GetNSections() uint
	// Append a section after this section.
	Append(name string,imported bool) *Section
	// Add a parameter to this section.
	Append2(p *Parameter) *Parameter
	// Add a parameter to this section.
	AppendParameter(name string, valuestr string, comments *Comment,imported bool) *Parameter
	// Add a section to this section.
	AppendSection(name string, imported bool)
	// Set a parameter's value
	SetValue(name string,value string,quote byte) error
	// Set a parameter's value to the given pointer.
	SetValuePtr(name string,value string,quote byte) error
	// Set a parameter's value to the given pointer on the given index.
	SetValuePtrOnIndex(name, value string, i uint, quote byte) error
	SetValueInFormat(name string,i int,format string,src any) error
	GetName() string
	// Get the first value for any array parameter.
	FindFirstParameter() *Parameter       // Get pointer to first parameter
	// Get a pointer to a Parameter.
	FindParameter(name string, searchParents bool) *Parameter
	FindNextParameter() *Parameter        // Get pointer to next parameter
	FindSection(name string) *Section      // Get pointer to a Section.
	GetFirstSection() *Section            // Get a pointer to a first section.
	GetNext() *Section                    // Get pointer to next section
	SetNext(p *Section)                   // Set pointer to next section
	GetFirst() *Parameter                 // Get pointer to first parameter.
	GetLast() *Parameter                  // Get pointer to last parameter.
	GetSelectedParameterName() string     // Get the name of the selected parameter.
  GetSelectedParameter() *Parameter     // Get the selected parameter.
	GetNParents() uint                    // Get the number of parents.
	GetParent(n uint) *Section            // Get the nth parent.
	GetParentName(n uint) string          // Get the name of the nth parent.
	RemoveMissingParent(i uint)           // Remove the nth parent.
	SelectFirstParameter()                // Select the first parameter.
	SelectParameter(p *Parameter)         // Select the given parameter.
	SelectParameterByName(name string) error// Select the parameter by name.
	GetParameter(name string, searchParents bool) *Parameter // Get a parameter by name.
	// Get array of values for a parameter name.
	GetValueArray(name string) []string
	// C-style strings
	GetValue(name string, i uint) string     // Get parameter value for a section name.
	GetValues(name string) string           // Get values for a parameter name.
  // Boolean
	GetValueBool(name string,i uint,tval string, fval string) (bool,error)// Get a boolean value for a parameter name.
	// Integer
	ScanValue(name string,i int, format string, dest any) error
  ClearParameters() error                // Clear all parameters in this section.
	SetParentNames(name string)            // First pass.
	SetParentSection(i uint, p *Section)   // Second pass.
	MakeShallowCopyOf(src *Section)        // Shallow copy of a section.
	Print(w io.Writer) (int64,error) 	
}
type Section struct{
  name        string                      // The name of the section.
	parentNames []string                     // The names of the parent sections.
	next        *Section                   // The next section in the list.
	parents     []*Section                  // The parent sections.
	nParents    uint                       // The number of parent in this section.
	nParameters uint                       // The number of Parameter objects.
	nSections   uint                       // The number of section objects.
	firstSection,lastSection *Section      // First and last sections in the list of sections.
	first,last  *Parameter                 // First and last parameters.
  current     *Parameter                 // The current parameter.
	comments    *Comment                   // The comments associated with this section.
	cfg         *Configuration             // The configuration object that owns this section.
	// ----------------------------------- //
	// If copy==true, then this section is a copy of another Section object, only
	// the name was allocated. This is used for Section references. All other
	// pointers, except next, are copies snd should not be deleted. The next
	// pointer is not a copy, because the list of referenced Sections in a Section
	// object will be different from the list of Sections in a Configuration object.
	// It should never be used to delete anything.
	// ----------------------------------- //
	copy        bool                       // True if is a copy of another section.
	isimported  bool                       // True if was imported.
}                                        
// ========================= // Configuration // ===============================
// A class to store the entire configuration file.
// ============================================================================
type ConfigurationAPI interface{
  NewConfiguration(ext string) *Configuration // Constructor with default extension.
	Reconfigure() error                    // Prepare to re-read the data file.
	SetDirectory(dir string)               // Set the directory for the configuration file.
	SetFilename(name string)               // Set the filename for the configuration file.
	SetDefaultExtension(ext string)        // Set the default extension.
	GetPathname() string                   // Get the pathname of the configuration file.
	GetImportedPathname() string           // Get the imported pathname of the configuration file.
	GetDirectory() string                  // Get the directory of the configuration file.
	GetFilename() string                   // Get the filename of the configuration file.
	SaveComments(flag bool)                // Enable or disable saving comments.	
	IgnoreImports(flag bool)              // Enable skipping import for file editing.
	NewFile(filename string)               // Create a new file.
	ReadFile(                             // Read the file from disk.
	  filename,section string,             // The name of the file to read.
		importing bool) error                // True if importing.
	WriteFile(filename string) error        // Write the file to disk.
	AppendSection(                        // Append a section to the file.
	  section string,                      // Name of new section.
		comments *Comment,                  // Comments to add.
		imported bool) *Section             // True if imported.
	FindSection(name string) *Section      // Find a section by name.
	FindFirstParameter() *Parameter       // Find first parameter in current section.
	GetFirstSection() *Section            // Get first section in the list.
	GetLastSection() *Section             // Get last section in the list.
	GetFirst()       *Section             // Get first section in the list.
	GetLast()        *Section             // Get last section in the list.
	GetSelectedSection() *Section         // Get current section.
	GetSection(name string) *Section       // Get a section by name.
	ClearParameters(section string) error   // Erase parameters in this section.
	SelectSection(section string) error // Select a section by name.
	SelectParameter(name string) error      // Select a parameter by name.
	
	splitCSVList(list string) []string     // Get an array of strings from a CSV list.
	detectSectionHeader(line string)(name,parents,fromfile string,err error)

  GetNextParameter() *Parameter         // Get next parameter in the list.
	GetParameter(name string, searchParents bool) *Parameter
	GetSectionName() string                // Get the name of the current section.
	GetNextParameterValues(vals [][]string,q []string) (name []string, nValues int, values [][]string,quotes []string,err error) // Get next parameter in the list.
  GetNextParameterValues2(vals [][]string) (name []string, values []string,err error)
  
	SetValue(name,valuestr string,quote byte) error // Set a value of a section.
	SetValueBySection(section,name string, i uint, value string) error
	SetValueInFormat(name string,val any,format string) error // Set a value in a given format.
  SetArrayValue(name,valuestr string,i uint,quote byte) error
	SetArrayValueBySection(section,name string, i uint, value string) error
	SetArrayValueBool(parameter string, i uint, value bool, tval string,fval string) error
	SetArrayValueInFormat(name string,idx uint,val any,format string) error
	
	GetValue(name string) string       // Get a string parameter from the selected section.
	GetValues(name string) string      // Get source string for parameter.
	GetValueByIndex(name string, i uint) string // Get a value by index for a parameter.
	GetValueBySection(section string, parameter string) string // Get value of named section.
	GetValueBySectionAndIndex(section, name string, i uint) string
	GetValueBool(parameter string, value *bool) error // Get a boolean parameter from the selected section.
	ScanValue(i int, fmt string, dest any) error
  GetNParameters(section string) uint   // Get number of parameters in a section.
	GetNValues(name string) uint          // Get number of values for a parameter of this section.
	GetSelectedSectionName() string       // Get the name of the selected section.
	GetSelectedSectionParentName() string // Get the name of the selected section's parent.
	GetFirstSectionName() string          // Get the name of the first section.
  Print(w io.Writer) (int64,error)
 // private methods.
 initialize()                           // Initialize the cfg object (noop for now).
 deleteAll()                            // Delete all data structures.
 saveComment(buf string, imports bool)  // Store a comment.



} 

type Configuration struct{
  path       string                     // The path to the configuration file.
	importpath string                     // The path to the import file.
	ext        string
	first,last *Section                   // First and last sections in the list of sections.
	current    *Section                   // The current section.
	firstComment,lastComment   *Comment   // Place to put comments at end of the file.
	saveComments bool                     // True if saving comments.
	ignoreImports bool                    // True if ignoring import statements.
	canWrite     bool                     // Set to false if did not read whole file.
	log          logger.Log               // The logger object.             
}
