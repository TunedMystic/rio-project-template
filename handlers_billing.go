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
	"app/views"

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

		// Idempotency: skip an event we've already fully processed. Stripe may
		// re-deliver an event even after a 200. Check before applying; record
		// only after a successful apply (so a failed apply is retried, not lost).
		if event.ID != "" {
			done, err := store.IsEventProcessed(r.Context(), event.ID)
			if err != nil {
				rio.LogError(err)
				return err
			}
			if done {
				w.WriteHeader(http.StatusOK)
				return nil
			}
		}

		switch event.Type {
		case "checkout.session.completed":
			uid, _ := strconv.ParseInt(event.UserID, 10, 64)
			product, ok := Conf.ProductByKey(event.ProductKey)
			if ok && product.Kind == config.OneTime {
				if uid != 0 {
					if err := store.GrantEntitlement(r.Context(), uid, event.ProductKey); err != nil {
						rio.LogError(err)
						return err
					}
				}
			} else if uid != 0 && event.CustomerID != "" {
				// Subscription checkout: ensure the customer is linked (status
				// itself arrives via customer.subscription.updated).
				if err := store.SetStripeCustomerID(r.Context(), uid, event.CustomerID); err != nil {
					rio.LogError(err)
					return err
				}
			}
		case "customer.subscription.updated", "customer.subscription.deleted":
			if event.CustomerID != "" {
				if err := store.UpdateSubscription(r.Context(), event.CustomerID, event.Status, event.CurrentPeriodEnd); err != nil {
					rio.LogError(err)
					return err
				}
			}
		}
		if event.ID != "" {
			if err := store.RecordEvent(r.Context(), event.ID); err != nil {
				rio.LogError(err)
				return err
			}
		}
		w.WriteHeader(http.StatusOK)
		return nil
	}
	return rio.MakeHandler(fn)
}

func HandlePremium() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "Premium")
		return render(w, http.StatusOK, views.Premium(Conf.PageDataFor(account(r)), meta))
	}
	return rio.MakeHandler(fn)
}

func HandleGuide() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "The Guide")
		return render(w, http.StatusOK, views.Guide(Conf.PageDataFor(account(r)), meta))
	}
	return rio.MakeHandler(fn)
}
