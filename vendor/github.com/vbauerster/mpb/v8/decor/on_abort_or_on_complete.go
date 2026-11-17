package decor

// OnAbortOrOnComplete wrap decorator.
// Displays provided message on abort or on complete event.
//
//	`decorator` Decorator to wrap
//	`message` message to display
func OnAbortOrOnComplete(decorator Decorator, message string) Decorator {
	return OnAbort(OnComplete(decorator, message), message)
}

// OnAbortMetaOrOnCompleteMeta wrap decorator.
// Provided fn is supposed to wrap output of given decorator
// with meta information like ANSI escape codes for example.
// Primary usage intention is to set SGR display attributes.
//
//	`decorator` Decorator to wrap
//	`fn` func to apply meta information
func OnAbortMetaOrOnCompleteMeta(decorator Decorator, fn func(string) string) Decorator {
	return OnAbortMeta(OnCompleteMeta(decorator, fn), fn)
}
