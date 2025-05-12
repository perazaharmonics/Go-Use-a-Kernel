package logger

type Log interface {
	Inf(msg string, args ...interface{}) bool // Info log
	Deb(msg string, args ...interface{}) bool // Debug log
	War(msg string, args ...interface{}) bool // Warning log
	Err(msg string, args ...interface{}) bool // Error log
	Fat(msg string, args ...interface{}) bool // Fatal log
	ExitLog(msg string, args ...interface{})  // Exit log
	Shutdown() error                          // Shutdown the logger
}
