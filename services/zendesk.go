package services

import (
	"encoding/json"
	"net/http"
	"time"
)

type ZendeskClient struct {
	Subdomain string
	APIToken  string
	Email     string
}

type Ticket struct {
	ID    int      `json:"id"`
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
	SLA   SLAInfo  `json:"sla"`
	// Add other fields...
}

type SLAInfo struct {
	PolicyName string `json:"policy_name"`
	BreachTime string `json:"breach_time"`
}

func NewZendeskClient(subdomain, email, apiToken string) *ZendeskClient {
	return &ZendeskClient{
		Subdomain: subdomain,
		APIToken:  apiToken,
		Email:     email,
	}
}

func (z *ZendeskClient) GetTickets() ([]Ticket, error) {
	url := "https://" + z.Subdomain + ".zendesk.com/api/v2/tickets.json"
	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(z.Email+"/token", z.APIToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Tickets []Ticket `json:"tickets"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result.Tickets, err
}

// Add functions to handle SLAs, tags, etc.
