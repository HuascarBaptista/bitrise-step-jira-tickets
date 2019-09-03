package main

import (
	"encoding/base64"
	"fmt"
	"github.com/HuascarBaptista/bitrise-step-jira-tickets/jira"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-steputils/stepconf"
	"os"
)

// Config ...
type Config struct {
	UserName string `env:"user_name,required"`
	APIToken string `env:"api_token,required"`
	BaseURL  string `env:"base_url,required"`
	Projects string `env:"projects,required"`
	Status   string `env:"status,required"`
	Labels   string `env:"labels,required"`
}

func main() {
	var cfg Config
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Issue with input: %s", err)
	}

	stepconf.Print(cfg)
	fmt.Println()

	encodedToken := generateBase64APIToken(cfg.UserName, cfg.APIToken)
	client := jira.NewClient(encodedToken, cfg.BaseURL)
	jiraTicket := jira.Ticket{Projects: cfg.Projects, Status: cfg.Status, Labels: cfg.Labels}
	if err := client.GetJiraTickets(jiraTicket); err != nil {
		failf("Getting tickets failed with error: %s", err)
	}
}

func generateBase64APIToken(userName string, apiToken string) string {
	v := userName + `:` + apiToken
	return base64.StdEncoding.EncodeToString([]byte(v))
}

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}
