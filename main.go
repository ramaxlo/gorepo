package main

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

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
