package events

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDogfoodingEventLifecycleAndDLQ(t *testing.T) {
	dlq := NewDeadLetterQueue(3)
	bus := NewEventBus(dlq)
	ctx := context.Background()

	// 1. Test CLIENT_REGISTERED -> Create crm.lead in Odoo CRM
	regEvt := ClientRegisteredEvent{
		EventID:       "evt_reg_12345678",
		EventType:     EventClientRegistered,
		TenantID:      "tenant_acme",
		CustomerEmail: "owner@acme.com",
		CustomerName:  "Acme Corp",
		Country:       "AR",
		Channel:       "SANDBOX_TRIAL",
		Timestamp:     time.Now(),
	}

	err := bus.PublishClientRegistered(ctx, regEvt)
	if err != nil {
		t.Fatalf("PublishClientRegistered failed: %v", err)
	}

	if bus.GetTenantStatus("tenant_acme") != "TRIAL_ACTIVE" {
		t.Errorf("Expected status TRIAL_ACTIVE, got %s", bus.GetTenantStatus("tenant_acme"))
	}
	if bus.GetCRMLeadID("owner@acme.com") == "" {
		t.Errorf("Expected valid leadID in Odoo CRM, got empty")
	}

	// 2. Test TRIAL_EXPIRED -> Generate Odoo Invoice & Payment Link
	expEvt := TrialExpiredEvent{
		EventID:        "evt_exp_99998888",
		EventType:      EventTrialExpired,
		TenantID:       "tenant_acme",
		TokensConsumed: 52400,
		FreeQuota:      50000,
		BillableTokens: 2400,
		AmountDueUSD:   15.50,
		Timestamp:      time.Now(),
	}

	err = bus.PublishTrialExpired(ctx, expEvt)
	if err != nil {
		t.Fatalf("PublishTrialExpired failed: %v", err)
	}
	if bus.GetInvoiceID("tenant_acme") == "" {
		t.Errorf("Expected valid InvoiceID in Odoo Invoicing, got empty")
	}

	// 3. Test PAYMENT_SUCCESS -> Activate Subscription to PROD_ACTIVE in < 1,000ms
	payEvt := PaymentSuccessEvent{
		EventID:       "evt_pay_77776666",
		EventType:     EventPaymentSuccess,
		TenantID:      "tenant_acme",
		InvoiceID:     bus.GetInvoiceID("tenant_acme"),
		AmountPaidUSD: 15.50,
		PaymentMethod: "STRIPE",
		Timestamp:     time.Now(),
	}

	err = bus.PublishPaymentSuccess(ctx, payEvt)
	if err != nil {
		t.Fatalf("PublishPaymentSuccess failed: %v", err)
	}
	if bus.GetTenantStatus("tenant_acme") != "PROD_ACTIVE" {
		t.Errorf("Expected status PROD_ACTIVE, got %s", bus.GetTenantStatus("tenant_acme"))
	}

	// 4. Test Dead Letter Queue (DLQ) Exception Routing
	mockErr := errors.New("Odoo ERP Connection Timeout")
	dlq.EnqueueFailedEvent(ctx, "evt_fail_0001", EventTrialExpired, expEvt, mockErr, 3)

	dlqItems := dlq.GetDLQItems()
	if len(dlqItems) != 1 {
		t.Fatalf("Expected 1 DLQ item, got %d", len(dlqItems))
	}
	if dlqItems[0].EventID != "evt_fail_0001" {
		t.Errorf("Expected DLQ EventID evt_fail_0001, got %s", dlqItems[0].EventID)
	}
}
