package main

import (
	"bytes"
	"fmt"
	"os"
	"runtime/debug"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func cmdVersion(ctx *cli.Context) error {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fmt.Errorf("Fail to read build info")
	}

	var rev string
	var isDirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				isDirty = true
			}
		}
	}
	buf := bytes.NewBuffer(nil)
	fmt.Fprintf(buf, "%s", rev[:8])
	if isDirty {
		fmt.Fprintf(buf, "-dirty")
	}

	fmt.Printf("%s\n", buf.String())

	return nil
}

var CmdVersion = cli.Command{
	Name:   "version",
	Usage:  "Display version",
	Action: cmdVersion,
}

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "15:04:05",
	})
	log.SetLevel(log.InfoLevel)

	app := &cli.App{
		Usage: "A stripped-down version of repo tool, written in Go",
		Commands: []*cli.Command{
			&CmdInit,
			&CmdSync,
			&CmdStatus,
			&CmdInfo,
			&CmdVersion,
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Usage:   "Enable debugging messages",
				Aliases: []string{"d"},
				Action: func(ctx *cli.Context, v bool) error {
					if v {
						log.SetLevel(log.DebugLevel)
					}
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
