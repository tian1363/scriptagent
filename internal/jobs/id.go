package jobs

import (
	"crypto/rand"
	"encoding/hex"
)

func newID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return "job_" + hex.EncodeToString(buf[:])
}
