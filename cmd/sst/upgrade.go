package main

import (
	"github.com/sst/sst/v3/cmd/sst/cli"
	"github.com/sst/sst/v3/internal/util"
)

func CmdUpgrade(c *cli.Cli) error {
	return util.NewReadableError(nil, "Forum SST uses immutable reviewed artifacts. Install an approved Remark-Inc/sst artifact; sst upgrade is disabled in this fork.")
}
