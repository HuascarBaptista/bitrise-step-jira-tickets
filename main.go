package main

import (
	"encoding/base64"
	"fmt"
	"github.com/HuascarBaptista/bitrise-step-jira-tickets/jira"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-steputils/stepconf"
	"os"
	"strconv"
)

// Config ...
type Config struct {
	UserName          string `env:"user_name,required"`
	APIToken          string `env:"api_token,required"`
	AllowEmptyVersion string `env:"allow_empty_version,required"`
	FixVersion        string `env:"fix_version"`
	BaseURL           string `env:"base_url,required"`
	Projects          string `env:"projects,required"`
	Status            string `env:"status,required"`
	Labels            string `env:"labels,required"`
}

func main() {
	var cfg Config
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Issue with input: %s", err)
	}

	allowEmptyVersion, error := strconv.ParseBool(cfg.AllowEmptyVersion)
	if error != nil {
		failf("Allow empty version unset")
	}
	if !allowEmptyVersion && cfg.FixVersion == "" {
		failf("Allow empty version set as false but Fix version is empty")
	}
	stepconf.Print(cfg)
	fmt.Println()

	encodedToken := generateBase64APIToken(cfg.UserName, cfg.APIToken)
	client := jira.NewClient(encodedToken, cfg.BaseURL)
	jiraTicket := jira.Ticket{Projects: cfg.Projects, Status: cfg.Status, Labels: cfg.Labels, AllowEmptyVersion: allowEmptyVersion, FixVersion: cfg.FixVersion}
	if err := client.GetJiraTickets(jiraTicket); err != nil {
		failf("Getting tickets failed with error: %s", err)
	}
	log.Infof("Finishin AllStep")
	os.Exit(0)
}

func generateBase64APIToken(userName string, apiToken string) string {
	v := userName + `:` + apiToken
	return base64.StdEncoding.EncodeToString([]byte(v))
}

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}
