package decor

var _ Decorator = anyDecorFunc{}

// Any decorator.
// Converts DecorFunc into Decorator.
//
//	`fn` DecorFunc callback
//	`wcc` optional WC config
func Any(fn DecorFunc, wcc ...WC) Decorator {
	return anyDecorFunc{initWC(wcc...), fn}
}

type anyDecorFunc struct {
	WC
	fn DecorFunc
}

func (d anyDecorFunc) Decor(s Statistics) (string, int) {
	return d.Format(d.fn(s))
}
