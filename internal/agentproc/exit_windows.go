package agentproc

import "os"

// killedBy is never a signal: a Windows process ends with an exit code
// alone, a killed one's too.
func killedBy(*os.ProcessState) (string, bool) { return "", false }
