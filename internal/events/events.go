package events

import (
	"time"
)

type EventType string

const (
	EventClientRegistered EventType = "CLIENT_REGISTERED"
	EventTrialExpired      EventType = "TRIAL_EXPIRED"
	EventPaymentSuccess   EventType = "PAYMENT_SUCCESS"
)

type ClientRegisteredEvent struct {
	EventID       string    `json:"event_id"`
	EventType     EventType `json:"event_type"`
	TenantID      string    `json:"tenant_id"`
	CustomerEmail string    `json:"customer_email"`
	CustomerName  string    `json:"customer_name"`
	Country       string    `json:"country"`
	Channel       string    `json:"channel"`
	Timestamp     time.Time `json:"timestamp"`
}

type TrialExpiredEvent struct {
	EventID        string    `json:"event_id"`
	EventType      EventType `json:"event_type"`
	TenantID       string    `json:"tenant_id"`
	TokensConsumed int64     `json:"tokens_consumed"`
	FreeQuota      int64     `json:"free_quota"`
	BillableTokens int64     `json:"billable_tokens"`
	AmountDueUSD   float64   `json:"amount_due_usd"`
	Timestamp      time.Time `json:"timestamp"`
}

type PaymentSuccessEvent struct {
	EventID       string    `json:"event_id"`
	EventType     EventType `json:"event_type"`
	TenantID      string    `json:"tenant_id"`
	InvoiceID     string    `json:"invoice_id"`
	AmountPaidUSD float64   `json:"amount_paid_usd"`
	PaymentMethod string    `json:"payment_method"`
	Timestamp     time.Time `json:"timestamp"`
}
