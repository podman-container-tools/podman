package mpb

import (
	"io"
	"slices"

	"github.com/vbauerster/mpb/v8/decor"
)

// BarOption is a func option to alter default behavior of a bar.
type BarOption func(*bState)

// PrependDecorators let you inject decorators to the bar's left side.
func PrependDecorators(decorators ...decor.Decorator) BarOption {
	i := 0
	for _, d := range decorators {
		if d != nil {
			decorators[i] = d
			i++
		}
	}
	return func(s *bState) {
		s.decorGroups[0] = slices.Clip(decorators[:i])
	}
}

// AppendDecorators let you inject decorators to the bar's right side.
func AppendDecorators(decorators ...decor.Decorator) BarOption {
	i := 0
	for _, d := range decorators {
		if d != nil {
			decorators[i] = d
			i++
		}
	}
	return func(s *bState) {
		s.decorGroups[1] = slices.Clip(decorators[:i])
	}
}

// BarID sets bar id.
func BarID(id int) BarOption {
	return func(s *bState) {
		s.id = id
	}
}

// BarWidth sets bar width independent of the container.
func BarWidth(width int) BarOption {
	return func(s *bState) {
		s.reqWidth = width
	}
}

// BarQueueAfter puts this (being constructed) bar into the queue.
// BarPriority will be inherited from the argument bar.
// When argument bar completes or aborts queued bar replaces its place.
func BarQueueAfter(bar *Bar) BarOption {
	return func(s *bState) {
		s.waitFor = bar
	}
}

// BarRemoveOnComplete removes both bar's filler and its decorators on
// complete event. This one is ineffective if PopCompletedMode ContainerOption
// is enabled.
func BarRemoveOnComplete() BarOption {
	return func(s *bState) {
		s.rmOnComplete = true
	}
}

// BarFillerClearOnComplete clears bar's filler on complete event.
// It's shortcut for BarFillerOnComplete("").
func BarFillerClearOnComplete() BarOption {
	return BarFillerOnComplete("")
}

// BarFillerOnComplete replaces bar's filler with message, on complete event.
func BarFillerOnComplete(message string) BarOption {
	return BarFillerMiddleware(func(base BarFiller) BarFiller {
		return BarFillerFunc(func(w io.Writer, st decor.Statistics) error {
			if st.Completed {
				_, err := io.WriteString(w, message)
				return err
			}
			return base.Fill(w, st)
		})
	})
}

// BarFillerClearOnAbort clears bar's filler on abort event.
// It's shortcut for BarFillerOnAbort("").
func BarFillerClearOnAbort() BarOption {
	return BarFillerOnAbort("")
}

// BarFillerOnAbort replaces bar's filler with message, on abort event.
func BarFillerOnAbort(message string) BarOption {
	return BarFillerMiddleware(func(base BarFiller) BarFiller {
		return BarFillerFunc(func(w io.Writer, st decor.Statistics) error {
			if st.Aborted {
				_, err := io.WriteString(w, message)
				return err
			}
			return base.Fill(w, st)
		})
	})
}

// BarFillerMiddleware provides a way to augment the underlying BarFiller.
func BarFillerMiddleware(middle func(BarFiller) BarFiller) BarOption {
	if middle == nil {
		return nil
	}
	return func(s *bState) {
		s.filler = middle(s.filler)
	}
}

// BarPriority sets bar's priority. Zero is highest priority, i.e. bar
// will be on top. This option isn't effective with `BarQueueAfter` option.
func BarPriority(priority int) BarOption {
	return func(s *bState) {
		s.priority = priority
	}
}

// BarExtender is deprecated use BarTopExtender or BarBtmExtender instead.
func BarExtender(filler BarFiller, top bool) BarOption {
	return func(s *bState) {
		s.rowProducers = makeRowProducers(s.draw, top, filler)
	}
}

// BarTopExtender extends a bar with arbitrary lines above.
// Each BarFiller represent one line so it should write '\n' no more than once.
// For example if there is need to extend a bar by 2 lines, provide 2 fillers
// and so on. If BarFiller writes more than one line then whole output is going
// to be corrupted. This option cannot be used together with BarBtmExtender.
func BarTopExtender(fillers ...BarFiller) BarOption {
	return func(s *bState) {
		s.rowProducers = makeRowProducers(s.draw, true, fillers...)
	}
}

// BarBtmExtender extends a bar with arbitrary lines below.
// Each BarFiller represent one line so it should write '\n' no more than once.
// For example if there is need to extend a bar by 2 lines, provide 2 fillers
// and so on. If BarFiller writes more than one line then whole output is going
// to be corrupted. This option cannot be used together with BarTopExtender.
func BarBtmExtender(fillers ...BarFiller) BarOption {
	return func(s *bState) {
		s.rowProducers = makeRowProducers(s.draw, false, fillers...)
	}
}

// BarFillerTrim removes leading and trailing space around the underlying BarFiller.
func BarFillerTrim() BarOption {
	return func(s *bState) {
		s.trimSpace = true
	}
}

// BarNoPop disables bar pop out of container. Effective when
// PopCompletedMode of container is enabled.
func BarNoPop() BarOption {
	return func(s *bState) {
		s.noPop = true
	}
}

// BarOptional will return provided option only when cond is true.
func BarOptional(option BarOption, cond bool) BarOption {
	if cond {
		return option
	}
	return nil
}

// BarOptOn will return provided option only when predicate evaluates to true.
func BarOptOn(option BarOption, predicate func() bool) BarOption {
	if predicate() {
		return option
	}
	return nil
}

// BarFuncOptional will call option and return its value only when cond is true.
func BarFuncOptional(option func() BarOption, cond bool) BarOption {
	if cond {
		return option()
	}
	return nil
}

// BarFuncOptOn will call option and return its value only when predicate evaluates to true.
func BarFuncOptOn(option func() BarOption, predicate func() bool) BarOption {
	if predicate() {
		return option()
	}
	return nil
}
