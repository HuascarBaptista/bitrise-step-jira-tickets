package jira

import (
	"encoding/json"
	"fmt"
	"github.com/bitrise-io/go-steputils/tools"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/urlutil"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"unicode"
)

const (
	apiEndPoint = "/rest/api/3/search?jql="
)

// Client ...
type Client struct {
	token   string
	client  *http.Client
	headers map[string]string
	baseURL string
}

type JiraTicketsResponse struct {
	Issues []struct {
		Key    string `json:"key"`
		Fields struct {
			Summary string `json:"summary"`
		} `json:"fields"`
	} `json:"issues"`
}

type response struct {
	err           error
	ticketNames   string
	ticketSummary string
}

type Ticket struct {
	Projects string
	Status   string
	Platform string
}

var customPrivateFieldsForPlatform = []string{"cf[11722]", "cf[10200]", "cf[11251]", "cf[11075]"}

func (resp response) String() string {
	respValue := map[bool]string{true: colorstring.Green("SUCCESS"), false: colorstring.Red("FAILED")}[resp.err == nil]
	return fmt.Sprintf("Gettings tickets to - %s - : %s : %s", respValue, resp.ticketNames, resp.ticketSummary)
}

// -------------------------------------
// -- Public methods

// NewClient ...
func NewClient(token, baseURL string) *Client {
	return &Client{
		token:  token,
		client: &http.Client{},
		headers: map[string]string{
			"Authorization": `Basic ` + token,
			"Content-Type":  "application/json",
		},
		baseURL: baseURL,
	}
}

func (client *Client) GetJiraTickets(jiraTicket Ticket) error {
	ch := make(chan response)
	go client.getJiraTickets(jiraTicket, ch)
	counter := 0
	var responses []response
	for resp := range ch {
		counter++
		log.Infof("Printing results")
		log.Printf(resp.String())
		if resp.err != nil {
			responses = append(responses, resp)
		}
		log.Infof("Finishin printing results")
		break
	}

	if len(responses) > 0 {
		fmt.Println()
		log.Infof("Errors during getting tickets:")

		for _, respErr := range responses {
			log.Warnf("Error during getting tickets to - %s", respErr.err.Error())
		}

		fmt.Println()
	}
	return map[bool]error{true: fmt.Errorf("some tickets were failed to be posted at Jira")}[len(responses) > 0]
}

func (client *Client) getJiraTickets(jiraTicket Ticket, ch chan response) {
	urlEncoded := getJiraSearchUrlEncoded(jiraTicket)
	requestURL, err := urlutil.Join(client.baseURL, apiEndPoint+urlEncoded)
	log.Infof("URL to retrieve Jira Tickets: %s", requestURL)
	if err != nil {
		ch <- response{err, "", ""}
		return
	}
	request, err := createRequest(http.MethodGet, requestURL, client.headers)
	if err != nil {
		ch <- response{err, "", ""}
		return
	}

	requestBytes, err := httputil.DumpRequest(request, true)
	if err != nil {
		ch <- response{err, "", ""}
		return
	}
	log.Debugf("Request: %v", string(requestBytes))

	// Perform request
	jiraTicketsResponseTwo := JiraTicketsResponse{}
	jsonResponse, body, err := client.performRequest(request, JiraTicketsResponse{})
	if err := mapstructure.Decode(jsonResponse, &jiraTicketsResponseTwo); err != nil {
		ch <- response{err, "", ""}
	}
	log.Debugf("Body: %s", string(body))
	var ticketsName string = ""
	var ticketsSlack string = ""
	var ticketsHyperLinkSlack string = ""
	for _, issue := range jiraTicketsResponseTwo.Issues {
		ticketsName += issue.Key + "|"
		description := issue.Fields.Summary
		ticketsHyperLinkSlack += "<" + client.baseURL + "/browse/" + issue.Key + "|" + issue.Key + ">: " + description + "\n"
	}
	if len(ticketsName) > 0 {
		ticketsName = ticketsName[:len(ticketsName)-1]
	}
	println(ticketsName)
	if err := tools.ExportEnvironmentWithEnvman("JIRA_TICKETS_SLACK", ticketsHyperLinkSlack); err != nil {
		ch <- response{fmt.Errorf("failed to export JIRA_TICKETS_SLACK, error: %s", err), "", ""}
		return
	}

	if err := tools.ExportEnvironmentWithEnvman("JIRA_TICKETS_NAME", ticketsName); err != nil {
		ch <- response{fmt.Errorf("failed to export JIRA_TICKETS_NAME, error: %s", err), "", ""}
		return
	}

	ch <- response{err, ticketsName, ticketsSlack}
}

func createRequest(requestMethod string, url string, headers map[string]string) (*http.Request, error) {
	var err error
	req, err := http.NewRequest(requestMethod, url, nil)
	if err != nil {
		return nil, err
	}
	addHeaders(req, headers)
	return req, nil
}

func removeAccents(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	output, _, e := transform.String(t, s)
	if e != nil {
		panic(e)
	}
	return output
}

func (client *Client) performRequest(req *http.Request, requestResponse interface{}) (interface{}, []byte, error) {
	response, err := client.client.Do(req)
	if err != nil {
		// On error, any Response can be ignored
		return nil, nil, fmt.Errorf("failed to perform request, error: %s", err)
	}

	// The client must close the response body when finished with it
	defer func() {
		if cerr := response.Body.Close(); cerr != nil {
			log.Warnf("Failed to close response body, error: %s", cerr)
		}
	}()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body, error: %s", err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode > http.StatusMultipleChoices {
		return nil, nil, fmt.Errorf("Response status: %d - Body: %s", response.StatusCode, string(body))
	}

	// Parse JSON body
	if requestResponse != nil {
		if err := json.Unmarshal([]byte(body), &requestResponse); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal response (%s), error: %s", body, err)
		}

		LogDebugPretty(&requestResponse)
	}
	return requestResponse, body, nil
}

func getJiraSearchUrlEncoded(ticket Ticket) string {
	var result = ""
	stringToConcat := "\",\""
	projects := convertToJQLInSentence(ticket.Projects, stringToConcat)
	status := convertToJQLInSentence(ticket.Status, stringToConcat)
	result = concatProjects(projects, result)
	result = concatStatus(status, result)
	result = concatPlatform(ticket.Platform, result)
	result += concatOrder()
	t := &url.URL{Path: result}
	return t.String()
}

func concatPlatform(platform string, result string) string {
	if platform != "" {
		if result != "" {
			result += " and "
		}
		result += " ("
		for i := range customPrivateFieldsForPlatform {
			result += customPrivateFieldsForPlatform[i] + "=\"" + platform + "\""
			if i != (len(customPrivateFieldsForPlatform) - 1) {
				result += " or "
			}
		}
		result += ")"
	}
	return result
}

func concatOrder() string {
	return " order by project"
}

func concatStatus(status string, result string) string {
	if status != "" {
		if result != "" {
			result += " and "
		}
		result += "status in (\"" + status + ")"
	}
	return result
}

func concatProjects(projects string, result string) string {
	if projects != "" {
		result += "project in (\"" + projects + ")"
	}
	return result
}

func convertToJQLInSentence(valueToTransfor string, stringToConcat string) string {
	if len(valueToTransfor) > 0 {
		return strings.Join(strings.Split(valueToTransfor, `|`), stringToConcat) + "\""
	}
	return ""
}

func addHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

func LogDebugPretty(v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println("error:", err)
	}

	log.Debugf("Response: %+v\n", string(b))
}
