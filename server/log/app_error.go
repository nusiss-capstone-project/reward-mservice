package log

import (
	"context"
	"errors"

	"github.com/nusiss-capstone-project/reward-mservice/server/errs"
)

func LogAppError(ctx context.Context, err error, msg string, keysAndValues ...any) {
	if err == nil {
		return
	}
	logger := WithContext(ctx)
	fields := append(keysAndValues, "error", err.Error())

	var appErr *errs.AppError
	if errors.As(err, &appErr) {
		fields = append(fields, "code", appErr.Code)
		if appErr.Code == errs.CodeInternalError {
			logger.Errorw(msg, fields...)
			return
		}
		logger.Warnw(msg, fields...)
		return
	}
	logger.Errorw(msg, fields...)
}
