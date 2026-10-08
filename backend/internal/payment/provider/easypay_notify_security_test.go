package provider

import (
	"context"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestEasyPayRejectsCheckoutSignatureReuse(t *testing.T) {
	e := &EasyPay{config: map[string]string{"pid": "1001", "pkey": "synthetic-merchant-key"}}
	checkout := map[string]string{
		"money": "12.34", "name": "Synthetic product", "notify_url": "https://synthetic.invalid/notify",
		"out_trade_no": "SYNTHETIC_ORDER", "pid": "1001", "type": "alipay",
		"return_url": "https://synthetic.invalid/payment/result?order_id=1&out_trade_no=SYNTHETIC_ORDER&status=success&trade_status=TRADE_SUCCESS",
	}
	sign := easyPaySign(checkout, e.config["pkey"])
	for _, swallowed := range []bool{false, true} {
		t.Run(map[bool]string{false: "order-only-keys", true: "allowed-key-swallowing"}[swallowed], func(t *testing.T) {
			callback := cloneStringMap(checkout)
			callback["return_url"] = "https://synthetic.invalid/payment/result?order_id=1&out_trade_no=SYNTHETIC_ORDER&status=success"
			callback["trade_status"] = tradeStatusSuccess
			if swallowed {
				callback["name"] += "&notify_url=" + callback["notify_url"]
				callback["pid"] += "&return_url=" + callback["return_url"]
				delete(callback, "notify_url")
				delete(callback, "return_url")
			}
			require.Equal(t, sign, easyPaySign(callback, e.config["pkey"]), "fixture must reproduce signing ambiguity")
			callback["sign"] = sign
			callback["sign_type"] = signTypeMD5
			_, err := e.VerifyNotification(context.Background(), notifyForm(callback).Encode(), nil)
			require.Error(t, err, "checkout signature cannot authenticate a payment")
		})
	}
}

func notifyForm(params map[string]string) url.Values {
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	return values
}

func TestEasyPayNotificationBoundary(t *testing.T) {
	e := &EasyPay{config: map[string]string{"pid": "1001", "pkey": "synthetic-merchant-key"}}
	base := map[string]string{
		"pid": "1001", "trade_no": "SYNTHETIC_TRADE", "out_trade_no": "SYNTHETIC_ORDER",
		"money": "12.34", "name": "Synthetic product", "type": "alipay", "trade_status": tradeStatusSuccess,
	}
	for _, tc := range []struct {
		name string
		edit func(map[string]string)
	}{
		{"unknown-empty-key", func(p map[string]string) { p["return_url"] = "" }},
		{"wrong-merchant", func(p map[string]string) { p["pid"] = "1002" }},
		{"missing-merchant", func(p map[string]string) { delete(p, "pid") }},
		{"empty-order", func(p map[string]string) { p["out_trade_no"] = "" }},
		{"unsupported-sign-type", func(p map[string]string) { p["sign_type"] = "SHA256" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := cloneStringMap(base)
			tc.edit(params)
			params["sign"] = easyPaySign(params, e.config["pkey"])
			_, err := e.VerifyNotification(context.Background(), notifyForm(params).Encode(), nil)
			require.Error(t, err)
		})
	}
	base["sign"] = easyPaySign(base, e.config["pkey"])
	form := notifyForm(base)
	for _, key := range []string{"pid", "out_trade_no", "money", "trade_status", "sign"} {
		t.Run("duplicate-"+key, func(t *testing.T) {
			duplicate := notifyForm(base)
			duplicate.Add(key, base[key])
			_, err := e.VerifyNotification(context.Background(), duplicate.Encode(), nil)
			require.Error(t, err)
		})
	}
	notification, err := e.VerifyNotification(context.Background(), form.Encode(), nil)
	require.NoError(t, err)
	require.Equal(t, payment.ProviderStatusSuccess, notification.Status)
	require.Equal(t, "SYNTHETIC_ORDER", notification.OrderID)
	require.Equal(t, 12.34, notification.Amount)
	base["trade_status"] = "TRADE_FAILED"
	base["sign"] = easyPaySign(base, e.config["pkey"])
	notification, err = e.VerifyNotification(context.Background(), notifyForm(base).Encode(), nil)
	require.NoError(t, err)
	require.Equal(t, payment.ProviderStatusFailed, notification.Status)
}
