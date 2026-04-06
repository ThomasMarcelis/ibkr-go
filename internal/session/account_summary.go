package session

import "github.com/ThomasMarcelis/ibkr-go/internal/codec"

type accountSummaryPlan struct {
	request  codec.AccountSummaryRequest
	wildcard bool
	account  string
}

func newAccountSummaryPlan(reqID int, req AccountSummaryRequest) accountSummaryPlan {
	wildcard := req.Account == "" || req.Account == "All"
	account := req.Account
	if wildcard {
		account = ""
	}
	return accountSummaryPlan{
		request: codec.AccountSummaryRequest{
			ReqID:   reqID,
			Account: "All",
			Tags:    append([]string(nil), req.Tags...),
		},
		wildcard: wildcard,
		account:  account,
	}
}

func (p accountSummaryPlan) matches(account string) bool {
	return p.wildcard || p.account == account
}
