package main

import (
	_ "embed"
	"log"

	"github.com/fatih/color"
	"github.com/joho/godotenv"

	"github.com/stanley760/devops/devops-infra/http/humaerr"
	"github.com/stanley760/devops/devops-infra/utils"

	"github.com/stanley760/devops/devops-user/internal/cmd"
)

// banner is embedded from banner.txt at compile time by //go:embed, so no file
// read is needed at runtime; the image does not need to ship banner.txt either
// (a multi-stage build only has to COPY the binary).
//
//go:embed banner.txt
var banner string

// ui prints program output such as the banner and version info.
// It is a standalone log instance (Flags=0, output to color.Output): multi-line
// banners are not pushed askew by a timestamp prefix, the terminal gets proper
// colors, and redirection strips escape codes automatically.
var ui = log.New(color.Output, "", 0)

func main() {
	// Load config/.env early: ResolveVersion depends on APP_VERSION, while
	// config.Init (cobra.OnInitialize) only runs once cmd.Execute is called.
	// godotenv.Load does not override already-set OS environment variables,
	// so CI/container injection is safe.
	_ = godotenv.Load("config/.env")

	// huma error envelope wiring: must complete before any huma request is
	// handled. It overrides huma.NewError so 4xx/5xx responses emit the
	// BaseResponse envelope instead of problem+json.
	humaerr.Install()

	ver := utils.ResolveVersion()

	ui.Print(banner)

	cyan := color.New(color.FgCyan).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	ui.Printf(" %s: %s\n", cyan("devops-user Service Version"), green(ver))

	cmd.Execute(ver)
}
