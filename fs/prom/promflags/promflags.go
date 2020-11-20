package promflags

import (
	"github.com/rclone/rclone/cmd/serve/httplib/httpflags"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/prom"
	"github.com/spf13/pflag"
)

// Options set by command line flags
var (
	Opt = prom.DefaultOpt
)

// AddFlags adds the prometheus flags to the flagSet
func AddFlags(flagSet *pflag.FlagSet) {
	flags.BoolVarP(flagSet, &Opt.Enabled, "prometheus", "", false, "Enable the prometheus metrics http server.")
	flags.BoolVarP(flagSet, &Opt.UseRcServer, "prometheus-use-rc-server", "", false, "Ignore all http specific options except the path and use the rc http server for serving the metrics.")
	flags.StringVarP(flagSet, &Opt.Path, "prometheus-path", "", Opt.Path, "The URL path where the metrics are served. Must start with a /.")
	flags.StringVarP(flagSet, &Opt.WriteFile, "prometheus-write-file", "", Opt.WriteFile, "Write the metrics to this file.")
	flags.DurationVarP(flagSet, &Opt.WriteInterval, "prometheus-write-interval", "", Opt.WriteInterval, "Interval at which the metrics are written to file.")
	httpflags.AddFlagsPrefix(flagSet, "prometheus-", &Opt.HTTPOptions)
}
