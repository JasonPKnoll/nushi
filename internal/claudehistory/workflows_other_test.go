//go:build !unix

package claudehistory

// makeFifo is nil where there are no FIFOs: TestWorkflowFilesStayInside
// leaves that case out.
var makeFifo func(path string) error
