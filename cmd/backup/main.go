// Copyright 2021-2022 - offen.software <hioffen@posteo.de>
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"flag"
)

func main() {
	foreground := flag.Bool("foreground", false, "run the tool in the foreground")
	source := flag.String("source", "from environment", "source of backup to execute in command mode")
	profile := flag.String("profile", "", "collect runtime metrics and log them periodically on the given cron expression")
	flag.Parse()

	c := newCommand()
	if *foreground {
		opts := foregroundOpts{
			profileCronExpression: *profile,
		}
		c.must(c.runInForeground(opts))
	} else {
		opts := commandOpts{
			source: *source,
		}
		c.must(c.runAsCommand(opts))
	}
}
