//go:build integration

package repository

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPaymentPromotionPostgresTransactions(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	userRepo := NewUserRepository(client, integrationDB)
	redeemRepo := NewRedeemCodeRepository(client)
	redeemService := service.NewRedeemService(redeemRepo, userRepo, nil, NewRedeemCache(testRedis(t)), nil, client, nil, nil)
	svc := service.NewPaymentService(client, nil, nil, redeemService, nil, nil, userRepo, nil, nil)
	instance, err := client.PaymentProviderInstance.Create().SetProviderKey(payment.TypeAlipay).
		SetName("promotion transaction fixture").SetConfig("{}").SetSupportedTypes(payment.TypeAlipay).
		SetRefundEnabled(true).Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM payment_provider_instances WHERE id=$1`, instance.ID)
		require.NoError(t, err)
	})
	for _, tc := range []struct {
		name                        string
		credit, bonus, pay, balance float64
		currency                    string
		overflow                    bool
	}{
		{name: "bonus", credit: 16.8, bonus: 2.8, pay: 102.5},
		{name: "discount", credit: 14, bonus: 2.8, pay: 82},
		{name: "three decimal discount callback", credit: 10, bonus: 0, pay: 9.999, currency: "IQD"},
		{name: "balance overflow rolls back redemption", credit: 16.8, bonus: 2.8, pay: 102.5, balance: 999999999990, overflow: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			currency, status := "CNY", service.OrderStatusPaid
			if tc.currency != "" {
				currency, status = tc.currency, service.OrderStatusPending
			}
			key := fmt.Sprintf("promotion-%d", time.Now().UnixNano())
			user, err := client.User.Create().SetEmail(key + "@example.test").SetPasswordHash("hash").SetBalance(tc.balance).Save(ctx)
			require.NoError(t, err)
			t.Cleanup(func() {
				_, err := integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, user.ID)
				require.NoError(t, err)
			})
			t.Cleanup(func() {
				_, err := integrationDB.ExecContext(ctx, `DELETE FROM redeem_codes WHERE code=$1`, key)
				require.NoError(t, err)
			})
			order, err := client.PaymentOrder.Create().SetUserID(user.ID).SetUserEmail(user.Email).SetUserName(key).
				SetAmount(tc.credit).SetBonusAmount(tc.bonus).SetPayAmount(tc.pay).SetFeeRate(2.5).
				SetRechargeCode(key).SetOutTradeNo(key).SetPaymentType(payment.TypeAlipay).SetPaymentTradeNo(key).
				SetClientIP("127.0.0.1").SetSrcHost("fixture.example.test").
				SetProviderInstanceID(strconv.FormatInt(instance.ID, 10)).SetProviderKey(payment.TypeAlipay).
				SetProviderSnapshot(map[string]any{"schema_version": 2, "provider_instance_id": strconv.FormatInt(instance.ID, 10), "provider_key": payment.TypeAlipay, "currency": currency}).
				SetStatus(status).SetExpiresAt(time.Now().Add(time.Hour)).Save(ctx)
			require.NoError(t, err)
			t.Cleanup(func() {
				_, err := integrationDB.ExecContext(ctx, `DELETE FROM payment_audit_logs WHERE order_id=$1`, strconv.FormatInt(order.ID, 10))
				require.NoError(t, err)
				_, err = integrationDB.ExecContext(ctx, `DELETE FROM payment_orders WHERE id=$1`, order.ID)
				require.NoError(t, err)
			})
			persisted, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, tc.pay, persisted.PayAmount)
			if tc.currency != "" {
				err = svc.HandlePaymentNotification(ctx, &payment.PaymentNotification{OrderID: key, TradeNo: key, Amount: tc.pay, Status: payment.NotificationStatusSuccess}, payment.TypeAlipay)
			} else {
				err = svc.ExecuteBalanceFulfillment(ctx, order.ID)
			}
			if tc.overflow {
				require.Error(t, err)
				code, err := redeemRepo.GetByCode(ctx, key)
				require.NoError(t, err)
				require.Equal(t, service.StatusUnused, code.Status)
				fresh, err := client.User.Get(ctx, user.ID)
				require.NoError(t, err)
				require.Equal(t, tc.balance, fresh.Balance)
				return
			}
			require.NoError(t, err)
			require.NoError(t, svc.ExecuteBalanceFulfillment(ctx, order.ID))
			fresh, err := client.User.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, tc.credit, fresh.Balance, "retry must grant the stored total once")
			reloaded, err := svc.GetOrder(ctx, order.ID, user.ID)
			require.NoError(t, err)
			require.Equal(t, service.OrderStatusCompleted, reloaded.Status)
			require.Equal(t, tc.bonus, reloaded.BonusAmount)
			for _, fraction := range []float64{1, .5} {
				plan, result, err := svc.PrepareRefund(ctx, order.ID, tc.credit*fraction, "fixture", false, false)
				require.NoError(t, err)
				require.Nil(t, result)
				expected := decimal.NewFromFloat(tc.pay).Mul(decimal.NewFromFloat(fraction)).Round(int32(payment.CurrencyMaxFractionDigits(currency))).InexactFloat64()
				require.Equal(t, expected, plan.GatewayAmount, "refund must use the stored gateway ratio")
			}
		})
	}
}
