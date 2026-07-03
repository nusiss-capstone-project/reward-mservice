package data

type PaymentConfigVO struct {
	PayAddress     string `json:"pay_address"`
	VoucherType    string `json:"voucher_type"`
	PaymentAccount string `json:"payment_account"`
}
