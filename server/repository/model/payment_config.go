package model

type PaymentConfig struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	PayAddress     string `gorm:"type:varchar(128);uniqueIndex;not null"`
	VoucherType    string `gorm:"type:varchar(64);not null"`
	Unit           string `gorm:"type:varchar(32);not null"`
	PaymentAccount string `gorm:"type:varchar(128);not null"`
}

func (PaymentConfig) TableName() string {
	return "payment_configs"
}
