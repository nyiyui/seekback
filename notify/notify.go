//go:build !sdnotify

package notify

import "log"

func Notify(state string) (err error) {
	log.Printf("notify: %s", state)
	return nil
}
