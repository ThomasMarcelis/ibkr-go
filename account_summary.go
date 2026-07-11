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
	group := req.Group
	if group == "" {
		group = "All"
	}
	return accountSummaryPlan{
		request: codec.AccountSummaryRequest{
			ReqID:   reqID,
			Account: group,
			Tags:    req.Tags,
		},
		wildcard: req.AccountFilter == "",
		account:  req.AccountFilter,
	}
}

func (p accountSummaryPlan) matches(account string) bool {
	return p.wildcard || p.account == account
}
