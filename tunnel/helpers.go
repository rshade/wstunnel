package tunnel

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

// VV Used for versioning build
var VV string

// SetVV Used for versioning build
func SetVV(vv string) { VV = vv }

func writePid(file string) error {
	if file != "" {
		_ = os.Remove(file)
		pid := os.Getpid()
		f, err := os.Create(file)
		if err != nil {
			return fmt.Errorf("can't create pidfile %s: %v", file, err)
		}
		_, err = f.WriteString(strconv.Itoa(pid) + "\n")
		if err != nil {
			_ = f.Close()
			return fmt.Errorf("can't write to pidfile %s: %v", file, err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("failed to close pidfile: %v", err)
		}
	}
	return nil
}

const simpleTimeFormat = "2006-01-02 15:04:05"

func calcWsTimeout(log zerolog.Logger, tout int) time.Duration {
	var wsTimeout time.Duration
	if tout < 3 {
		wsTimeout = 3 * time.Second
	} else if tout > 600 {
		wsTimeout = 600 * time.Second
	} else {
		wsTimeout = time.Duration(tout) * time.Second
	}
	log.Info().Dur("timeout", wsTimeout).Msg("Setting WS keep-alive")
	return wsTimeout
}

// copy http headers over
func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
