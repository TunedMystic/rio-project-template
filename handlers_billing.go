// handlers_billing.go
package main

import (
	"io"
	"net/http"
	"strconv"

	"app/auth"
	"app/billing"
	"app/config"
	"app/database"

	"github.com/tunedmystic/rio"
)

func HandleCheckout(store *database.Store, bc billing.Client) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		if !requireCSRF(w, r) {
			return nil
		}
		user, _ := auth.UserFrom(r.Context())
		product, ok := Conf.ProductByKey(r.FormValue("product"))
		if !ok || !product.Available() {
			w.WriteHeader(http.StatusBadRequest)
			return nil
		}
		customerID, err := bc.EnsureCustomer(r.Context(), user.Email, user.ID, user.StripeCustomerID)
		if err != nil {
			return err
		}
		if user.StripeCustomerID == "" {
			if err := store.SetStripeCustomerID(r.Context(), user.ID, customerID); err != nil {
				return err
			}
		}
		url, err := bc.CreateCheckoutSession(r.Context(), billing.CheckoutInput{
			CustomerID:   customerID,
			PriceID:      product.PriceID,
			Subscription: product.Kind == config.Subscription,
			UserID:       user.ID,
			ProductKey:   product.Key,
			SuccessURL:   Conf.BaseURL + "/account/billing?status=success",
			CancelURL:    Conf.BaseURL + "/account/billing",
		})
		if err != nil {
			return err
		}
		http.Redirect(w, r, url, http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}

func HandlePortal(store *database.Store, bc billing.Client) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		if !requireCSRF(w, r) {
			return nil
		}
		user, _ := auth.UserFrom(r.Context())
		if user.StripeCustomerID == "" {
			http.Redirect(w, r, "/account/billing", http.StatusSeeOther)
			return nil
		}
		url, err := bc.CreatePortalSession(r.Context(), user.StripeCustomerID, Conf.BaseURL+"/account/billing")
		if err != nil {
			return err
		}
		http.Redirect(w, r, url, http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}

func HandleStripeWebhook(store *database.Store, bc billing.Client) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return nil
		}
		event, err := bc.VerifyWebhook(payload, r.Header.Get("Stripe-Signature"), Conf.StripeWebhookSecret)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return nil
		}

		switch event.Type {
		case "checkout.session.completed":
			uid, _ := strconv.ParseInt(event.UserID, 10, 64)
			product, ok := Conf.ProductByKey(event.ProductKey)
			if ok && product.Kind == config.OneTime {
				if uid != 0 {
					_ = store.GrantEntitlement(r.Context(), uid, event.ProductKey)
				}
			} else if uid != 0 && event.CustomerID != "" {
				// Subscription checkout: ensure the customer is linked (status
				// itself arrives via customer.subscription.updated).
				_ = store.SetStripeCustomerID(r.Context(), uid, event.CustomerID)
			}
		case "customer.subscription.updated", "customer.subscription.deleted":
			if event.CustomerID != "" {
				_ = store.UpdateSubscription(r.Context(), event.CustomerID, event.Status, event.CurrentPeriodEnd)
			}
		}
		w.WriteHeader(http.StatusOK)
		return nil
	}
	return rio.MakeHandler(fn)
}
