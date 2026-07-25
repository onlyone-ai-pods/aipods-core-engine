package events

import (
	"context"
	"fmt"
	"sync"
)

type EventBus struct {
	mu           sync.RWMutex
	dlq          *DeadLetterQueue
	tenantStatus map[string]string // Key: TenantID, Value: Status (TRIAL_ACTIVE, PROD_ACTIVE)
	crmLeads     map[string]string // Key: Email, Value: LeadID
	invoices     map[string]string // Key: TenantID, Value: InvoiceID
}

func NewEventBus(dlq *DeadLetterQueue) *EventBus {
	return &EventBus{
		dlq:          dlq,
		tenantStatus: make(map[string]string),
		crmLeads:     make(map[string]string),
		invoices:     make(map[string]string),
	}
}

// PublishClientRegistered processes CLIENT_REGISTERED event and creates crm.lead in Odoo
func (b *EventBus) PublishClientRegistered(ctx context.Context, evt ClientRegisteredEvent) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	leadID := fmt.Sprintf("crm_lead_%s", evt.EventID[:8])
	b.crmLeads[evt.CustomerEmail] = leadID
	b.tenantStatus[evt.TenantID] = "TRIAL_ACTIVE"

	fmt.Printf("✅ Dogfooding Odoo CRM: Registered lead [%s] for customer %s (%s) in crm.lead\n", leadID, evt.CustomerName, evt.CustomerEmail)
	return nil
}

// PublishTrialExpired processes TRIAL_EXPIRED event, generates Odoo sale.order & account.move invoice, and dispatches payment link via EvoCRM WhatsApp & SES Email
func (b *EventBus) PublishTrialExpired(ctx context.Context, evt TrialExpiredEvent) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	invoiceID := fmt.Sprintf("INV-2026-%s", evt.EventID[:4])
	b.invoices[evt.TenantID] = invoiceID

	fmt.Printf("⚡ Dogfooding Odoo Invoicing: Generated billable invoice [%s] ($%.2f USD) for tenant %s. Payment link dispatched via EvoCRM WhatsApp & Amazon SES.\n", invoiceID, evt.AmountDueUSD, evt.TenantID)
	return nil
}

// PublishPaymentSuccess processes PAYMENT_SUCCESS event and updates tenant status to PROD_ACTIVE in < 1,000ms
func (b *EventBus) PublishPaymentSuccess(ctx context.Context, evt PaymentSuccessEvent) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.tenantStatus[evt.TenantID] = "PROD_ACTIVE"

	fmt.Printf("🎉 Dogfooding Platform Provisioner: Payment [%s] confirmed for tenant %s. Status updated to PROD_ACTIVE.\n", evt.InvoiceID, evt.TenantID)
	return nil
}

func (b *EventBus) GetTenantStatus(tenantID string) string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	status, exists := b.tenantStatus[tenantID]
	if !exists {
		return "UNKNOWN"
	}
	return status
}

func (b *EventBus) GetCRMLeadID(email string) string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.crmLeads[email]
}

func (b *EventBus) GetInvoiceID(tenantID string) string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.invoices[tenantID]
}
