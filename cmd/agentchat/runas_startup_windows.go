//go:build windows

package main

import (
	"context"

	"github.com/Renaissance-Mind/agent-in-chat-feishu/config"
)

func runRunAsUserStartupChecks(_ context.Context, _ *config.Config) error {
	return nil
}
