// Copyright 2020 The Go-Guardian. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/shaj13/go-guardian/auth"
	"github.com/shaj13/go-guardian/auth/strategies/basic"
	"github.com/shaj13/go-guardian/auth/strategies/twofactor"
	"github.com/shaj13/go-guardian/otp"
)

// Usage:
// curl  -k http://127.0.0.1:8080/v1/book/1449311601 -u admin:admin -H "X-Example-OTP: 345515"

var authenticator auth.Authenticator

func main() {
	setupGoGuardian()
	router := mux.NewRouter()
	router.HandleFunc("/v1/book/{id}", middleware(http.HandlerFunc(getBookAuthor))).Methods("GET")
	log.Println("server started and listening on http://127.0.0.1:8080")
	http.ListenAndServe("127.0.0.1:8080", router)
}

func getBookAuthor(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	books := map[string]string{
		"1449311601": "Ryan Boyd",
		"148425094X": "Yvonne Wilson",
		"1484220498": "Prabath Siriwarden",
	}
	body := fmt.Sprintf("Author: %s \n", books[id])
	w.Write([]byte(body))
}

func setupGoGuardian() {
	authenticator = auth.New()

	basicStrategy := basic.AuthenticateFunc(validateUser)
	tfaStrategy := twofactor.Strategy{
		Parser:  twofactor.XHeaderParser("X-Example-OTP"),
		Manager: OTPManager{},
		Primary: basicStrategy,
	}

	authenticator.EnableStrategy(twofactor.StrategyKey, tfaStrategy)
}

func validateUser(ctx context.Context, r *http.Request, userName, password string) (auth.Info, error) {
	// here connect to db or any other service to fetch user and validate it.
	if userName == "admin" && password == "admin" {
		return auth.NewDefaultUser("medium", "1", nil, nil), nil
	}

	return nil, fmt.Errorf("Invalid credentials")
}

func middleware(next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("Executing Auth Middleware")
		user, err := authenticator.Authenticate(r)
		if err != nil {
			code := http.StatusUnauthorized
			http.Error(w, http.StatusText(code), code)
			return
		}
		log.Printf("User %s Authenticated\n", user.UserName())
		next.ServeHTTP(w, r)
	})
}

type OTPManager struct{}

func (OTPManager) Enabled(_ auth.Info) bool { return true }

func (OTPManager) Load(_ auth.Info) (twofactor.OTP, error) {
	// user otp configuration must be loaded from persistent storage
	key := otp.NewKey(otp.HOTP, "LABEL", "GXNRHI2MFRFWXQGJHWZJFOSYI6E7MEVA")
	ver := otp.New(key)
	return ver, nil
}

func (OTPManager) Store(_ auth.Info, otp twofactor.OTP) error {
	// persist user otp after verification
	return nil
}
