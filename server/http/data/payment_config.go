package data

type PaymentConfigVO struct {
	PayAddress     string `json:"pay_address"`
	VoucherType    string `json:"voucher_type"`
	Unit           string `json:"unit"`
	PaymentAccount string `json:"payment_account"`
}
