// +build debug

package node

import "log"

func debug(fmt string, args ...interface{}) {
	log.Printf(fmt, args...)
}