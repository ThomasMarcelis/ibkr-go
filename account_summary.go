package ibkr

import "github.com/ThomasMarcelis/ibkr-go/internal/codec"

type accountSummaryPlan struct {
	request  codec.AccountSummaryRequest
	wildcard bool
	account  string
}

func cloneAccountSummaryRequest(req AccountSummaryRequest) AccountSummaryRequest {
	req.Tags = append([]string(nil), req.Tags...)
	return req
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
			Tags:    req.Tags,
		},
		wildcard: wildcard,
		account:  account,
	}
}

func (p accountSummaryPlan) matches(account string) bool {
	return p.wildcard || p.account == account
}
