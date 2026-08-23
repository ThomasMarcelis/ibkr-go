package ibkr

import (
	"context"
	"log/slog"
	"slices"
	"sync"
)

const asyncLogCapacity = 64

type asyncLogCore struct {
	handler  slog.Handler
	records  chan asyncLogRecord
	stop     chan struct{}
	stopOnce sync.Once
}

type asyncLogHandler struct {
	core *asyncLogCore
	ops  []asyncLogOp
}

type asyncLogRecord struct {
	ctx    context.Context
	record slog.Record
	ops    []asyncLogOp
}

type asyncLogOp struct {
	attrs []slog.Attr
	group string
}

func newAsyncLogger(logger *slog.Logger) (*slog.Logger, func()) {
	core := &asyncLogCore{
		handler: logger.Handler(),
		records: make(chan asyncLogRecord, asyncLogCapacity),
		stop:    make(chan struct{}),
	}
	go core.run()
	return slog.New(asyncLogHandler{core: core}), func() {
		core.stopOnce.Do(func() { close(core.stop) })
	}
}

func (h asyncLogHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h asyncLogHandler) Handle(ctx context.Context, record slog.Record) error {
	entry := asyncLogRecord{ctx: ctx, record: record.Clone(), ops: slices.Clone(h.ops)}
	select {
	case <-h.core.stop:
	case h.core.records <- entry:
	default:
	}
	return nil
}

func (h asyncLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return asyncLogHandler{core: h.core, ops: append(slices.Clone(h.ops), asyncLogOp{attrs: slices.Clone(attrs)})}
}

func (h asyncLogHandler) WithGroup(name string) slog.Handler {
	return asyncLogHandler{core: h.core, ops: append(slices.Clone(h.ops), asyncLogOp{group: name})}
}

func (c *asyncLogCore) run() {
	for {
		select {
		case entry := <-c.records:
			handler := c.handler
			for _, op := range entry.ops {
				if op.group != "" {
					handler = handler.WithGroup(op.group)
				} else {
					handler = handler.WithAttrs(op.attrs)
				}
			}
			if handler.Enabled(entry.ctx, entry.record.Level) {
				_ = handler.Handle(entry.ctx, entry.record)
			}
		case <-c.stop:
			return
		}
	}
}
