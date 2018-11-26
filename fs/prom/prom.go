package prom

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ncw/rclone/fs"

	"github.com/ncw/rclone/cmd/serve/httplib"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Options contains options for the prometheus metrics server
type Options struct {
	HTTPOptions   httplib.Options
	Enabled       bool          // set to enable the server
	UseRcServer   bool          // set to use the rc http server
	Path          string        // the url path to use
	WriteFile     string        // write the metrics to the file
	WriteInterval time.Duration // interval at which metrics are written
}

// DefaultOpt is the default values used for Options
var DefaultOpt = Options{
	HTTPOptions:   httplib.DefaultOpt,
	Path:          "/metrics",
	WriteInterval: 30 * time.Second,
}

func init() {
	DefaultOpt.HTTPOptions.ListenAddr = "localhost:9100"
}

// Instance represents the configured metrics exports
type Instance struct {
	opt         *Options
	server      *httplib.Server
	writeTicker *time.Ticker
	writeExit   chan struct{}
}

// Start the prometheus metrics exports if configured
func Start(opt *Options) (*Instance, error) {
	inst := &Instance{
		opt:       opt,
		writeExit: make(chan struct{}, 1),
	}
	if err := inst.startServer(); err != nil {
		return inst, err
	}
	if err := inst.startWriter(); err != nil {
		return inst, err
	}
	return inst, nil
}

// Stop closes the server and stops the write ticker if configured
func (inst *Instance) Stop() {
	if inst == nil {
		return
	}
	if inst.server != nil {
		inst.server.Close()
		inst.server = nil
	}
	if inst.writeTicker != nil {
		inst.writeTicker.Stop()
		inst.writeTicker = nil
		<-inst.writeExit
	}
}

func (inst *Instance) startServer() error {
	if inst.opt.Enabled {
		if !strings.HasPrefix(inst.opt.Path, "/") {
			return fmt.Errorf("Configured Path %q does not start with a /", inst.opt.Path)
		}
		handler := promhttp.Handler()
		mux := http.DefaultServeMux
		if !inst.opt.UseRcServer {
			mux = http.NewServeMux()
			srv := httplib.NewServer(mux, &inst.opt.HTTPOptions)
			if err := srv.Serve(); err != nil {
				return err
			}
			inst.server = srv
		}
		mux.Handle(inst.opt.Path, handler)
	}
	return nil
}
func (inst *Instance) startWriter() error {
	if inst.opt.WriteFile != "" {
		t := inst.opt.WriteInterval
		if t <= 0 {
			t = math.MaxInt64
		}
		inst.writeTicker = time.NewTicker(t)
		go func() {
			i := 0
			for t := range inst.writeTicker.C {
				inst.writeMetrics(i, t)
				i++
			}
			inst.writeMetrics(i, time.Now())
			close(inst.writeExit)
		}()
	}
	return nil
}
func (inst *Instance) writeMetrics(i int, t time.Time) {
	p := os.Expand(inst.opt.WriteFile, func(v string) string {
		switch v {
		case "NOW":
			return strconv.FormatInt(t.Unix(), 10)
		case "NOW_NANO":
			return strconv.FormatInt(t.UnixNano(), 10)
		case "LOOP":
			return strconv.FormatInt(int64(i), 10)
		default:
			if strings.HasPrefix(v, "LOOP_MOD_") {
				m, err := strconv.Atoi(v[4:])
				if err == nil {
					return strconv.FormatInt(int64(i%m), 10)
				}
			}
			return os.Getenv(v)
		}
	})

	mts, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		fs.Logf("prometheus", "error gathering metrics: %s", err)
		return
	}

	fd, err := os.Create(p)
	if err != nil {
		fs.Logf("prometheus", "error creating metrics file %q: %s", p, err)
		return
	}
	defer func() {
		if err := fd.Close(); err != nil {
			fs.Logf("prometheus", "error closing metrics file %q: %s", p, err)
		}
	}()

	for _, m := range mts {
		_, err := fmt.Fprintln(fd, m.String())
		if err != nil {
			fs.Logf("prometheus", "error writing metrics file %q: %s", p, err)
			return
		}
	}
}
