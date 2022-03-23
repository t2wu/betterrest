package query

import (
	"log"
	"runtime"
)

func PrintFileAndLine(err error) {
	_, filepath, line, ok := runtime.Caller(2)
	if !ok {
		log.Println("PrintFileAndLine unable to print file and line number")
		return
	}
	// https://stackoverflow.com/questions/5947742/how-to-change-the-output-color-of-echo-in-linux
	log.Printf("[BetterQuery] \033[1;36m(%s:%d): %s\033[0m", filepath, line, err.Error())
}
