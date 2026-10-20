package main

import (
	_ "embed"
	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"github.com/stanley760/devops/devops-infra/http/humaerr"
	"github.com/stanley760/devops/devops-infra/utils"
	"github.com/stanley760/devops/devops-system/internal/cmd"
	"log"
)

// banner is embedded at compile time from banner.txt via //go:embed, so no
// file read is needed at runtime; the image does not need to carry
// banner.txt either (the multi-stage build only COPYs the binary).
//
//go:embed banner.txt
var banner string

// ui prints "program output" such as the banner and version information.
// A standalone log instance (Flags=0, writing to color.Output) keeps the
// multi-line banner from being skewed by a time prefix, renders colors
// correctly on a terminal, and strips escape codes automatically when
// redirected.
var ui = log.New(color.Output, "", 0)

func main() {
	// Load config/.env early: resolveVersion depends on APP_VERSION, while
	// config.Init (cobra.OnInitialize) does not run until cmd.Execute.
	// godotenv.Load does not override OS environment variables that are
	// already set, so CI/container injection is safe.
	_ = godotenv.Load("config/.env")

	// huma error envelope wiring (D1): must be done before any huma request
	// handling; overrides huma.NewError so 4xx/5xx responses emit the
	// BaseResponse envelope instead of problem+json.
	humaerr.Install()

	ver := utils.ResolveVersion()

	ui.Print(banner)

	cyan := color.New(color.FgCyan).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	ui.Printf(" %s: %s\n", cyan("devops-system Service Version"), green(ver))

	cmd.Execute(ver)
}
