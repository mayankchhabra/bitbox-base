package middleware

import (
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/digitalbitbox/bitbox-base/middleware/src/rpcmessages"
)

// The util.go file includes utillity functions for the Middleware.
// These are private and called by the middleware RPCs. Utillity
// functions like `mountFlashdrive` or `unmountFlashdrive` are
// reused in multiple RPCs.

// runBBBCmdScript runs the bbb-cmd.sh script.
// The script executes commands like for example mounting a USB drive, doing a backup and copying files.
func (middleware *Middleware) runBBBCmdScript(args []string) (outputLines []string, err error) {
	outputLines, err = runCommand(middleware.environment.GetBBBCmdScript(), args)
	return
}

// runBBBConfigScript runs the bbb-config.sh script.
// The script changes the system configuration in redis by setting or unsetting the appropriate keys.
// If necessary the affected services are restarted.
func (middleware *Middleware) runBBBConfigScript(args []string) (outputLines []string, err error) {
	outputLines, err = runCommand(middleware.environment.GetBBBConfigScript(), args)
	return
}

// runCommand runs the passed command and returns the combined stdout and stderr output.
// If the command could not be run, err is not nil.
func runCommand(command string, args []string) (combinedLines []string, err error) {
	cmd := exec.Command(command, args...)
	log.Printf("executing command: %s %s", command, strings.Join(args, " "))

	rawstdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		// no error handling here, just logging
		log.Printf("Error executing command '%s %s': '%s'", command, strings.Join(args, " "), err.Error())
	}

	combined := strings.TrimSuffix(string(rawstdoutStderr), "\n")
	combinedLines = strings.Split(combined, "\n")
	return combinedLines, err
}

func handleBBBScriptErrorCode(outputLines []string, err error, possibleErrors []rpcmessages.ErrorCode) rpcmessages.ErrorCode {
	// There are two possible types of errors handled here:
	// 1.   The script could not be found.
	// 2.   Script exited with `exit status 1`:
	// 	2.1. Script was not run as superuser. ErrorCode ErrorScriptNotSuperuser is expected as the last outputLine.
	// 	2.2. CMD Script was not run with correct parameters. ErrorCode ErrorCmdScriptInvalidArg is expected as the last outputLine.
	// 	2.3. Config Script was not run with correct parameters. ErrorCode ErrorConfigScriptInvalidArg is expected as the last outputLine.
	// 	2.4. One of the `possibleErrors` is expected as the last outputLine.
	// All other errors are unknow and not handled. ErrorUnexpected is returned as a last resort.

	if os.IsNotExist(err) {
		return rpcmessages.ErrorScriptNotFound
	} else if err.Error() == "error status 1" {

		if len(outputLines) == 0 {
			log.Println("Error: no log lines provided before exit with error status 1.")
			return rpcmessages.ErrorUnexpected
		}

		outputErrorCode := outputLines[len(outputLines)-1]

		// handling default errors the bbb-cmd and bbb-config scripts can return
		switch outputErrorCode {
		case string(rpcmessages.ErrorScriptNotSuperuser):
			return rpcmessages.ErrorScriptNotSuperuser // Script was not run as superuser.
		case string(rpcmessages.ErrorCmdScriptInvalidArg):
			return rpcmessages.ErrorCmdScriptInvalidArg // Invalid arguments passed to the bbb-cmd.sh script.
		case string(rpcmessages.ErrorConfigScriptInvalidArg):
			return rpcmessages.ErrorConfigScriptInvalidArg // Invalid arguments passed to the bbb-config.sh script.
		}

		// handling specific possible errors the executed part of the script can throw
		// e.g. flashdrive mounting errors when executing a flashdrive mount
		for _, possible := range possibleErrors {
			if outputErrorCode == string(possible) {
				return possible
			}
		}
	}

	log.Printf("Error: unhandled error '%s' with output '%s'", err.Error(), outputLines)
	return rpcmessages.ErrorUnexpected
}