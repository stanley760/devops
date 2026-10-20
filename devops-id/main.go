package main

import (
	_ "embed"
	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"github.com/stanley760/devops/devops-id/internal/cmd"
	"github.com/stanley760/devops/devops-infra/utils"
	"log"
)

//go:embed banner.txt
var banner string

// ui prints "program output" such as the banner and version info.
// A separate log instance (Flags=0, writing to color.Output) keeps multi-line banners
// from being pushed off-center by a time prefix, colors normally on a terminal,
// and strips escape sequences automatically when redirected.
var ui = log.New(color.Output, "", 0)

func main() {
	// Load config/.env early: resolveVersion depends on APP_VERSION,
	// while config.Init (cobra.OnInitialize) only runs when cmd.Execute is called.
	// godotenv.Load does not override already-set OS environment variables, so CI/container injection is safe.
	_ = godotenv.Load("config/.env")

	ver := utils.ResolveVersion()

	ui.Print(banner)

	cyan := color.New(color.FgCyan).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	ui.Printf(" %s: %s\n", cyan("devops-id Service Version"), green(ver))

	cmd.Execute(ver)
}
