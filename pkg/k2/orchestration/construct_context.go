package orchestration

import (
	"context"

	"github.com/klothoplatform/klotho/pkg/k2/model"
	"github.com/klothoplatform/klotho/pkg/logging"
	"github.com/klothoplatform/klotho/pkg/tui"
	"go.uber.org/zap"
)

func ConstructContext(ctx context.Context, construct model.URN) context.Context {
	ctx = logging.WithLogger(ctx, logging.GetLogger(ctx).With(zap.String("construct", construct.ResourceID)))
	if prog := tui.GetProgram(ctx); prog != nil {
		ctx = tui.WithProgress(ctx, &tui.TuiProgress{
			Prog:      prog,
			Construct: construct.ResourceID,
		})
	}
	return ctx
}
