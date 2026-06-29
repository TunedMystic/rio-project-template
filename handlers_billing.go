// handlers_billing.go
package main

import (
	"net/http"

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
