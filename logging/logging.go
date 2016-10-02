package logging

import (
	"log"
	"os"
)

// Logger is the SMTPD logger.
var Logger *log.Logger

func init() {
	Logger = log.New(os.Stderr, "", 0)
}
