package dao

import (
	"context"
	"errors"
	"sync"

	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"gorm.io/gorm"
)

type FinancePaymentDao interface {
	Create(ctx context.Context, tx *gorm.DB, payment *model.FinancePayment) error
	GetByPaymentIDAndDocID(ctx context.Context, paymentID, docID string) (*model.FinancePayment, error)
	ListByDocID(ctx context.Context, docID string) ([]*model.FinancePayment, error)
}

type FinancePaymentDaoImpl struct {
	db *gorm.DB
}

var (
	financePaymentOnce sync.Once
	financePaymentDao  FinancePaymentDao
)

func GetFinancePaymentDao() FinancePaymentDao {
	financePaymentOnce.Do(func() {
		financePaymentDao = &FinancePaymentDaoImpl{db: repository.DB}
	})
	return financePaymentDao
}

func (d *FinancePaymentDaoImpl) Create(ctx context.Context, tx *gorm.DB, payment *model.FinancePayment) error {
	err := dbFrom(d.db, tx).WithContext(ctx).Create(payment).Error
	if err != nil {
		log.WithContext(ctx).Errorw("create finance payment failed",
			"doc_id", payment.FinanceDocID,
			"payment_id", payment.PaymentID,
			"error", err,
		)
		return err
	}
	return nil
}

func (d *FinancePaymentDaoImpl) GetByPaymentIDAndDocID(ctx context.Context, paymentID, docID string) (*model.FinancePayment, error) {
	var payment model.FinancePayment
	err := d.db.WithContext(ctx).
		Where("payment_id = ? AND finance_doc_id = ?", paymentID, docID).
		First(&payment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.WithContext(ctx).Errorw("get finance payment failed",
			"payment_id", paymentID,
			"doc_id", docID,
			"error", err,
		)
		return nil, err
	}
	return &payment, nil
}

func (d *FinancePaymentDaoImpl) ListByDocID(ctx context.Context, docID string) ([]*model.FinancePayment, error) {
	var payments []*model.FinancePayment
	err := d.db.WithContext(ctx).
		Where("finance_doc_id = ?", docID).
		Order("created_at DESC").
		Find(&payments).Error
	if err != nil {
		log.WithContext(ctx).Errorw("list finance payments failed",
			"doc_id", docID,
			"error", err,
		)
		return nil, err
	}
	return payments, nil
}

func dbFrom(defaultDB, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return defaultDB
}
