package lobster

import "fmt"
import "net/url"
import "net/http"
import "runtime/debug"
import "runtime"
import "strings"
import "unicode"

const MIN_USERNAME_LENGTH = 3
const MAX_USERNAME_LENGTH = 128
const MIN_PASSWORD_LENGTH = 6
const MAX_PASSWORD_LENGTH = 512
const MAX_VM_NAME_LENGTH = 64

const SESSION_UID_LENGTH = 64
const SESSION_COOKIE_NAME = "lobsterSession"

const TIME_FORMAT = "2 January 2006 15:04:05 MST"
const DATE_FORMAT = "2 January 2006"
const MYSQL_TIME_FORMAT = "2006-01-02 15:04:05"

// billing constants
const BILLING_PRECISION = 1000000 // credit is in units of 1/BILLING_PRECISION -dollars
const BILLING_DISPLAY_DECIMALS = 3
const MINIMUM_CREDIT = BILLING_PRECISION

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func redirectMessage(w http.ResponseWriter, r *http.Request, target string, msg string) {
	http.Redirect(w, r, target + "?message=" + url.QueryEscape(msg), 303)
}

func isPrintable(s string) bool {
	for _, c := range s {
		if c == 0 || c > unicode.MaxASCII || !unicode.IsPrint(c) {
			return false
		}
	}
	return true
}

// Extracts IP address from http.Request.RemoteAddr (127.0.0.1:9999 -> 127.0.0.1)
func extractIP(ipport string) string {
	return strings.Split(ipport, ":")[0]
}

// Report should be true unless this error handler is being used in an error reporting function.
func errorHandler(w http.ResponseWriter, r *http.Request, report bool) {
	if re := recover(); re != nil {
		debug.PrintStack()

		if report {
			stackBytes := make([]byte, 8192)
			runtime.Stack(stackBytes, false)

			if r != nil {
				reportError(re.(error), fmt.Sprintf("failed on %s (%s)", r.URL.Path, r.RemoteAddr), string(stackBytes))
			} else {
				reportError(re.(error), "error", string(stackBytes))
			}
		}

		if w != nil {
			http.Error(w, "Encountered error.", http.StatusInternalServerError)
		}
	}
}

func gigaToBytes(x int) int64 {
	return int64(x) * 1024 * 1024 * 1024
}
